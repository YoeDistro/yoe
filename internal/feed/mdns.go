package feed

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/libp2p/zeroconf/v2"
)

const mdnsServiceType = "_yoe-feed._tcp"

// MDNSConfig holds the parameters for an mDNS advertisement.
type MDNSConfig struct {
	Instance string   // e.g. "yoe-myproj"
	Project  string   // project name for TXT record
	Path     string   // URL path component, e.g. "/myproj"
	Archs    []string // archs available, joined with comma in TXT
	Port     int
}

// MDNSAdvertisement is a running zeroconf registration. Stop() to unregister.
type MDNSAdvertisement struct {
	server *zeroconf.Server
}

// AdvertiseMDNS registers a _yoe-feed._tcp service. The hostname used in
// SRV records comes from the OS; zeroconf picks the local IPs automatically.
func AdvertiseMDNS(cfg MDNSConfig) (*MDNSAdvertisement, error) {
	if cfg.Instance == "" {
		return nil, fmt.Errorf("mDNS instance name is empty")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("mDNS port is zero")
	}
	txt := []string{
		"project=" + cfg.Project,
		"path=" + cfg.Path,
		"arch=" + strings.Join(cfg.Archs, ","),
	}
	srv, err := zeroconf.Register(cfg.Instance, mdnsServiceType, "local.", cfg.Port, txt, nil)
	if err != nil {
		return nil, fmt.Errorf("zeroconf.Register: %w", err)
	}
	return &MDNSAdvertisement{server: srv}, nil
}

// Stop unregisters the advertisement.
func (a *MDNSAdvertisement) Stop() {
	if a == nil || a.server == nil {
		return
	}
	a.server.Shutdown()
}

// MDNSResult is a discovered _yoe-feed._tcp instance.
type MDNSResult struct {
	Instance string
	Host     string // .local hostname from SRV
	Port     int
	Project  string
	Path     string
	Archs    []string
	IPs      []net.IP
}

// BrowseMDNS scans for _yoe-feed._tcp instances for up to timeout.
func BrowseMDNS(timeout time.Duration) ([]MDNSResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry, 16)
	if err := zeroconf.Browse(ctx, mdnsServiceType, "local.", entries); err != nil {
		return nil, fmt.Errorf("zeroconf.Browse: %w", err)
	}

	var results []MDNSResult
	for e := range entries {
		r := MDNSResult{
			Instance: e.Instance,
			Host:     e.HostName,
			Port:     e.Port,
		}
		r.IPs = append(r.IPs, e.AddrIPv4...)
		for _, kv := range e.Text {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			switch k {
			case "project":
				r.Project = v
			case "path":
				r.Path = v
			case "arch":
				if v != "" {
					r.Archs = strings.Split(v, ",")
				}
			}
		}
		results = append(results, r)
	}
	return results, nil
}

// URL constructs the http URL for this discovered feed.
func (r MDNSResult) URL() string {
	host := strings.TrimSuffix(r.Host, ".")
	return fmt.Sprintf("http://%s:%d%s", host, r.Port, r.Path)
}
