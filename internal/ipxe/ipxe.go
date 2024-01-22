package ipxe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"
)

var ipxeBootTemplate = template.Must(template.New("ipxeBoot").Parse(
	`#!ipxe
kernel {{.Kernel}} {{.KernelArgs}}
initrd {{.Initrd}}
boot`,
))

const defaultTTL = 30 * time.Second

// IPxeBootRequest is the request sent to an endpoint in order to determine boot parameters
type IPxeBootRequest struct {
	// MAC is the MAC address of the machine
	MAC string `json:"mac"`
	// Arch is the architecture of the machine
	Arch string `json:"arch"`
	// Serial
	Serial string `json:"serial"`
}

// IPxeResponse is the exected response to an IPxeRequest
type IPxeBootResponse struct {
	Kernel     string `json:"kernel"`
	KernelArgs string `json:"kernel_args"`
	Initrd     string `json:"initrd"`
}

func (r *IPxeBootResponse) Write(wr io.Writer) error {
	return ipxeBootTemplate.Execute(wr, r)
}

type IpxeBootRetriever interface {
	GetBootResponse(ctx context.Context, mac, arch, serial string) (*IPxeBootResponse, error)
	HttpHandle(w http.ResponseWriter, req *http.Request)
}

func NewIpxeBootRetriever(url string) IpxeBootRetriever {
	return &ipxeBootRetriever{
		client:        &http.Client{},
		url:           url,
		cacheResponse: ttlcache.New[string, *IPxeBootResponse](),
	}
}

type ipxeBootRetriever struct {
	// client is the HTTP client to use
	client *http.Client
	// url is the url to query
	url string
	// cache
	cacheResponse *ttlcache.Cache[string, *IPxeBootResponse]
	// dedup
	singleflight singleflight.Group
}

// GetBootRequest returns the boot request for a given mac address
func (r *ipxeBootRetriever) GetBootResponse(ctx context.Context, mac, arch, serial string) (*IPxeBootResponse, error) {
	item, err, _ := r.singleflight.Do(mac, func() (interface{}, error) {
		if existing := r.cacheResponse.Get(mac); existing != nil {
			return existing.Value(), nil
		}

		// transform reqItem.value to io.Reader
		req := IPxeBootRequest{
			MAC:    mac,
			Arch:   arch,
			Serial: serial,
		}
		jsonReq, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}

		// Send request
		reqCtx, cancelReqCtx := context.WithTimeout(ctx, 1*time.Second)
		defer cancelReqCtx()
		httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, r.url, bytes.NewReader(jsonReq))
		if err != nil {
			return nil, err
		}

		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, err := r.client.Do(httpReq)
		if err != nil {
			return nil, err
		}

		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %d", httpResp.StatusCode)
		}

		var resp IPxeBootResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return nil, err
		}

		r.cacheResponse.Set(mac, &resp, defaultTTL)

		return &resp, nil
	})

	if err != nil {
		return nil, err
	}

	return item.(*IPxeBootResponse), nil
}

// HTTP handler for the iPXE chain load script
func (r *ipxeBootRetriever) HttpHandle(w http.ResponseWriter, req *http.Request) {
	// Get mac from query param
	mac := req.URL.Query().Get("mac")
	serial := req.URL.Query().Get("serial")
	arch := req.URL.Query().Get("arch")

	zerolog.Ctx(req.Context()).Info().Str("mac", mac).Str("serial", serial).Str("arch", arch).Msg("sending boot request")

	resp, err := r.GetBootResponse(req.Context(), mac, arch, serial)
	if err != nil {
		zerolog.Ctx(req.Context()).
			Error().
			Err(err).
			Str("mac", mac).
			Str("serial", serial).
			Str("arch", arch).
			Msgf("failed to get boot response from %s", r.url)
		// Retry booting
		http.ResponseWriter(w).WriteHeader(http.StatusNotFound)
		return
	}
	zerolog.Ctx(req.Context()).Info().Str("mac", mac).Str("serial", serial).Str("arch", arch).Msg("transmitting boot response")

	if err := resp.Write(w); err != nil {
		zerolog.Ctx(req.Context()).Error().Err(err).Msg("failed to write boot response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
