package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-envconfig"
	"github.com/xvzf/gobootme/internal/ipxe"
	"github.com/xvzf/gobootme/internal/proxydhcp"
	"github.com/xvzf/gobootme/internal/tftp"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	IpxeBootEndpoint   string `env:"IPXE_BOOT_ENDPOINT"`
	ProxyDhcpInterface string `env:"PROXY_DHCP_INTERFACE, default=eth0"`
	HttpPort           int    `env:"HTTP_PORT, default=8082"`
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		zerolog.Ctx(r.Context()).
			Info().
			Str("method", r.Method).
			Str("url", r.URL.RequestURI()).
			Str("user_agent", r.UserAgent()).
			Dur("duration", time.Since(start)).
			Str("remote_addr", r.RemoteAddr).
			Msg("Handling request")
	})
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	ctx = logger.WithContext(ctx)

	defer cancel()
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("Could not parse config")
	}

	// Resolve interface IP
	var interfaceIP net.IP
	proxyInterface, err := net.InterfaceByName(config.ProxyDhcpInterface)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Msgf("Could not find interface %s", config.ProxyDhcpInterface)
	}
	addrs, err := proxyInterface.Addrs()
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Msgf("Could not find interface %s", config.ProxyDhcpInterface)
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			interfaceIP = ipv4
			break
		}
	}

	// listen to stop signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	// Bootstrap
	grp, ctx := errgroup.WithContext(ctx)

	// Launch ProxyDHCP server
	proxyDhcpHandleFunc := proxydhcp.HandlePkt(
		ctx,
		interfaceIP,
		fmt.Sprintf(
			"http://%s:%d/ipxe?mac=${mac}&buildarch=${buildarch}&serial=${serial}",
			interfaceIP.String(),
			config.HttpPort,
		),
	)
	proxyDhcpServer, err := server4.NewServer(config.ProxyDhcpInterface, nil, proxyDhcpHandleFunc)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("Could not create ProxyDHCP server")
	}
	grp.Go(func() error {
		zerolog.Ctx(ctx).Info().Msgf("Starting ProxyDHCP server on %s", config.ProxyDhcpInterface)
		return proxyDhcpServer.Serve()
	})
	grp.Go(func() error {
		<-ctx.Done()
		zerolog.Ctx(ctx).Info().Msg("Shutting down ProxyDHCP server")
		return proxyDhcpServer.Close()
	})

	// Start tftp server
	tftpServer := tftp.NewIpxeServer()
	grp.Go(func() error {
		zerolog.Ctx(ctx).Info().Msgf("Starting TFTP server on %s:69", interfaceIP.String())
		return tftpServer.ListenAndServe(interfaceIP.String() + ":69")
	})
	grp.Go(func() error {
		<-ctx.Done()
		zerolog.Ctx(ctx).Info().Msg("Shutting down TFTP server")
		tftpServer.Shutdown()
		return nil
	})

	// Launch iPXE boot endpoint
	zerolog.Ctx(ctx).Info().Msgf("iPXE boot config endpoint: %s", config.IpxeBootEndpoint)
	bootRetriever := ipxe.NewIpxeBootRetriever(config.IpxeBootEndpoint)
	mux := http.NewServeMux()
	mux.HandleFunc("/ipxe", bootRetriever.HttpHandle)

	ipxeServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", interfaceIP.String(), config.HttpPort),
		Handler: requestLogger(mux),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	grp.Go(func() error {
		zerolog.Ctx(ctx).Info().Msgf("Starting iPXE boot config server on %s:%d", interfaceIP.String(), config.HttpPort)
		return ipxeServer.ListenAndServe()
	})
	grp.Go(func() error {
		<-ctx.Done()
		zerolog.Ctx(ctx).Info().Msg("Shutting down iPXE boot config server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return ipxeServer.Shutdown(shutdownCtx)
	})

	// Wait for all goroutines to finish
	if err := grp.Wait(); err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("Error while running")
	}
}
