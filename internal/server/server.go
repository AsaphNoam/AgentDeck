package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/store"
)

// shutdownTimeout bounds graceful shutdown after the context is cancelled.
const shutdownTimeout = 5 * time.Second

// Server owns the HTTP lifecycle and its dependencies.
type Server struct {
	store *store.Store
	cfg   store.Config
	log   *slog.Logger
}

// New constructs a Server. The config supplies the port; the store backs all
// data handlers; the logger is used by middleware and handlers.
func New(st *store.Store, cfg store.Config, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{store: st, cfg: cfg, log: log}
}

// Start binds 127.0.0.1:{cfg.Port}, asserts the listener is loopback, serves
// until ctx is cancelled, then shuts down gracefully. It blocks until shutdown
// completes or a fatal serve error occurs.
func (s *Server) Start(ctx context.Context) error {
	addr, err := LocalAddr(s.cfg.Port)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	// Defense in depth: refuse anything that is not loopback.
	if err := assertLoopback(ln.Addr()); err != nil {
		ln.Close()
		return err
	}

	srv := &http.Server{Handler: s.routes()}
	s.log.Info("dashboard listening", "addr", "http://"+ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		s.log.Info("dashboard shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
