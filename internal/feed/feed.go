package feed

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Config configures a Server.
type Config struct {
	RepoDir  string   // root of the served file tree (project's repo/)
	BindAddr string   // e.g. "0.0.0.0:8765" or "127.0.0.1:0" for ephemeral
	Project  string   // project name (used for default URL path and TXT)
	Archs    []string // archs present under repo/<project>/
	Instance string   // mDNS instance name; default "yoe-<project>"
	NoMDNS   bool     // skip mDNS advertisement entirely
	HostName string   // override .local hostname (default: os.Hostname() + ".local")
	LogW     io.Writer
}

// Server is a feed: HTTP serving + optional mDNS advertisement.
type Server struct {
	http *HTTPServer
	mdns *MDNSAdvertisement
	cfg  Config
	host string
}

// Start brings up the HTTP server and (unless NoMDNS) registers an mDNS
// advertisement. Returns once both are ready.
func Start(cfg Config) (*Server, error) {
	if cfg.RepoDir == "" {
		return nil, fmt.Errorf("RepoDir is empty")
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf("Project is empty")
	}
	if cfg.LogW == nil {
		cfg.LogW = io.Discard
	}
	if cfg.Instance == "" {
		cfg.Instance = "yoe-" + cfg.Project
	}

	host := cfg.HostName
	if host == "" {
		h, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("os.Hostname: %w", err)
		}
		host = strings.TrimSuffix(h, ".local") + ".local"
	}

	httpSrv, err := StartHTTP(cfg.RepoDir, cfg.BindAddr, cfg.LogW)
	if err != nil {
		return nil, err
	}

	s := &Server{http: httpSrv, cfg: cfg, host: host}

	if !cfg.NoMDNS {
		adv, err := AdvertiseMDNS(MDNSConfig{
			Instance: cfg.Instance,
			Project:  cfg.Project,
			Path:     "/" + cfg.Project,
			Archs:    cfg.Archs,
			Port:     httpSrv.Port(),
		})
		if err != nil {
			httpSrv.Stop()
			return nil, err
		}
		s.mdns = adv
	}

	return s, nil
}

// URL returns the user-facing feed URL using the .local hostname.
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s:%d/%s", s.host, s.http.Port(), s.cfg.Project)
}

// Addr returns the bound address (host:port) for direct-IP access.
func (s *Server) Addr() string { return s.http.Addr() }

// Port returns the bound port.
func (s *Server) Port() int { return s.http.Port() }

// Host returns the .local hostname used in URLs.
func (s *Server) Host() string { return s.host }

// Stop tears down mDNS first (so no new client discovers a feed about
// to disappear), then drains HTTP.
func (s *Server) Stop() error {
	if s.mdns != nil {
		s.mdns.Stop()
	}
	return s.http.Stop()
}
