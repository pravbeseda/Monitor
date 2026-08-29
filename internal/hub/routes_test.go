package hub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/hub"
	"github.com/pravbeseda/monitor/internal/storage"
)

type discard struct{}

func (discard) SaveIngest(context.Context, storage.Ingest) error    { return nil }
func (discard) States(context.Context) ([]storage.NodeState, error) { return nil, nil }
func (discard) Close() error                                        { return nil }

func routes(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("MONITOR_TOKEN_LAPTOP_A", strings.Repeat("synthetic-", 4))

	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "nodes:\n  laptop-a:\n    class: laptop\n    token_env: MONITOR_TOKEN_LAPTOP_A\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return hub.Routes(cfg, discard{}, time.Now)
}

func TestIngestIsMountedOnItsVersionedPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	routes(t).ServeHTTP(rec, req)

	// No token, so ingest answers 401 — what matters here is that it answered at all.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want ingest to handle the path", rec.Code)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nowhere", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	routes(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
