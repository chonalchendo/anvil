// Package ui serves a read-only, dark, loopback web view of the vault.
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

const freshnessEvery = 2 * time.Second

type server struct {
	v     *core.Vault
	db    *index.DB
	res   resolver
	md    markdown
	pages pages
	files assets

	mu      sync.Mutex
	checked time.Time
}

// newServer wires the shared state every handler reads.
func newServer(v *core.Vault, db *index.DB) (*server, error) {
	files, err := loadAssets()
	if err != nil {
		return nil, fmt.Errorf("loading static assets: %w", err)
	}
	p, err := loadPages(files)
	if err != nil {
		return nil, err
	}
	res := resolver{v: v}
	return &server{v: v, db: db, res: res, md: newMarkdown(res), pages: p, files: files}, nil
}

// Handler returns the route table. Each follow-on view adds one line here.
// A method pattern makes the mux answer 405 for any non-GET/HEAD request on a
// known path and 404 for an unknown one.
func Handler(v *core.Vault, db *index.DB) (http.Handler, error) {
	s, err := newServer(v, db)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /artifact/{key}", s.artifact)
	mux.HandleFunc("GET /static/{file}", s.files.serve)
	return mux, nil
}

// Serve answers on addr until ctx ends. addr must be a loopback address.
func Serve(ctx context.Context, v *core.Vault, db *index.DB, addr string, out io.Writer) error {
	if err := requireLoopback(addr); err != nil {
		return err
	}
	h, err := Handler(v, db)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(out, "anvil ui: http://%s\n", ln.Addr())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && (host == "localhost" || net.ParseIP(host).IsLoopback()) {
		return nil
	}
	return errfmt.NewStructured("ui_addr_not_loopback").
		Set("addr", addr).
		Set("hint", "bind 127.0.0.0/8, ::1 or localhost; the view has no auth")
}

// refresh reindexes when the vault drifted, at most once per freshnessEvery.
func (s *server) refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.checked) < freshnessEvery {
		return
	}
	s.checked = time.Now()
	err := s.db.CheckFreshness(s.v.Root)
	if err == nil {
		return
	}
	var stale *index.StaleError
	if !errors.Is(err, index.ErrLastReindexUnset) && !errors.As(err, &stale) {
		slog.Warn("index freshness check failed", "err", err)
		return
	}
	if _, err := s.db.Reindex(s.v.Root); err != nil {
		slog.Warn("reindex failed", "err", err)
	}
}

func (s *server) home(w http.ResponseWriter, _ *http.Request) {
	s.pages.render(w, "home", nil)
}
