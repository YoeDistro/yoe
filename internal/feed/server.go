// Package feed serves a yoe project's apk repository over HTTP and
// optionally advertises it on mDNS so devices and `yoe deploy` can
// discover it.
package feed

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPServer is the bare HTTP layer of a feed: an http.Server bound to
// a TCP listener, serving a directory tree.
type HTTPServer struct {
	listener net.Listener
	server   *http.Server

	stopOnce sync.Once
	stopErr  error
}

// StartHTTP listens on bindAddr and serves files rooted at repoDir.
// bindAddr is a host:port string; pass ":0" or "host:0" for an ephemeral
// port (read it back via Addr()). logW receives one line per request
// (method, path, status, bytes, duration); pass io.Discard to silence.
func StartHTTP(repoDir, bindAddr string, logW io.Writer) (*HTTPServer, error) {
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", bindAddr, err)
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(repoDir))
	mux.Handle("/", logHandler(fs, logW))

	s := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = s.Serve(ln)
	}()

	return &HTTPServer{listener: ln, server: s}, nil
}

// Addr returns the actual listening address, e.g. "127.0.0.1:8765".
func (s *HTTPServer) Addr() string {
	return s.listener.Addr().String()
}

// Port returns just the port number for use when constructing URLs that
// pair the port with a different (mDNS) hostname.
func (s *HTTPServer) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// Stop shuts the server down, draining in-flight requests for up to
// 5 seconds. Safe to call multiple times.
func (s *HTTPServer) Stop() error {
	s.stopOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.stopErr = s.server.Shutdown(ctx)
	})
	return s.stopErr
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func logHandler(next http.Handler, w io.Writer) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: rw}
		next.ServeHTTP(lrw, r)
		fmt.Fprintf(w, "%s %s %d %d %s\n",
			r.Method, r.URL.Path, lrw.status, lrw.bytes, time.Since(start).Round(time.Millisecond))
	})
}
