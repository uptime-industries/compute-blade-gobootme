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
	"github.com/rs/zerolog/log"
	"github.com/sethvargo/go-envconfig"
	"github.com/xvzf/gobootme/internal/proxydhcp"
	"github.com/xvzf/gobootme/internal/tftp"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	LogLevel             string `env:"LOG_LEVEL, default=info"`
	LogMode              string `env:"LOG_MODE, default=console"`
	EnableProxyDhcp      bool   `env:"ENABLE_PROXY_DHCP, default=true"`
	ProxyDhcpInterface   string `env:"PROXY_DHCP_INTERFACE, default=eth0"`
	IpxeBootEndpointAuto bool   `env:"IPXE_BOOT_ENDPOINT_AUTO, default=true"`
	IpxeBootEndpoint     string `env:"IPXE_BOOT_ENDPOINT"`
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
	var logger zerolog.Logger
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal().Err(err).Msg("Could not parse environment variables")
	}

	// Setup logger
	if config.LogMode == "json" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}
	switch config.LogLevel {
	case "debug":
		logger = logger.Level(zerolog.DebugLevel)
	case "info":
		logger = logger.Level(zerolog.InfoLevel)
	case "warn":
		logger = logger.Level(zerolog.WarnLevel)
	case "error":
		logger = logger.Level(zerolog.ErrorLevel)
	}

	ctx = logger.WithContext(ctx)

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

	if config.IpxeBootEndpointAuto {
		config.IpxeBootEndpoint = fmt.Sprintf("http://%s:8080/boot.ipxe", interfaceIP.String())
	}

	proxyDhcpHandleFunc := proxydhcp.HandlePkt(
		ctx,
		interfaceIP,
		config.IpxeBootEndpoint,
	)
	if config.EnableProxyDhcp {
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
	} else {
		zerolog.Ctx(ctx).Info().Msg("ProxyDHCP server is disabled")
	}

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

	// Wait for all goroutines to finish
	if err := grp.Wait(); err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("Error while running")
	}
}
