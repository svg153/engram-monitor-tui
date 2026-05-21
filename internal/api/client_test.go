package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/svg153/engram-monitor-tui/internal/model"
)

func TestHealthAndStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"engram","status":"ok","version":"0.1.0"}`))
		case "/stats":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"total_sessions":2,"total_observations":3,"total_prompts":1,"projects":["alpha"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	stats, err := client.Stats(context.Background())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if health.Status != "ok" || stats.TotalSessions != 2 {
		t.Fatalf("unexpected decoded values: %#v %#v", health, stats)
	}
}

func TestSearchAndMergeProjects(t *testing.T) {
	project := "alpha"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if got := r.URL.Query().Get("q"); got != "hello" {
				t.Fatalf("expected query hello, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":1,"session_id":"s1","type":"decision","title":"hello","content":"world","project":"alpha","scope":"project","created_at":"2026-05-21T10:00:00Z","updated_at":"2026-05-21T10:00:00Z"}]`))
		case "/projects/migrate":
			if r.Method != http.MethodPost {
				t.Fatalf("expected post")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	results, err := client.Search(context.Background(), model.SearchParams{Q: "hello"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || projectOf(results[0]) != project {
		t.Fatalf("unexpected search results: %#v", results)
	}
	if err := client.MergeProjects(context.Background(), "old", "new"); err != nil {
		t.Fatalf("merge: %v", err)
	}
}

func TestExportAndErrorDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/export":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"1"}`))
		case "/prompts/7":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad prompt id"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	raw, err := client.Export(context.Background(), "")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(raw) != `{"version":"1"}` {
		t.Fatalf("unexpected export payload: %s", string(raw))
	}
	err = client.DeletePrompt(context.Background(), 7)
	if err == nil || !strings.Contains(err.Error(), "bad prompt id") {
		t.Fatalf("expected decoded api error, got %v", err)
	}
}

func projectOf(obs model.Observation) string {
	if obs.Project == nil {
		return ""
	}
	return *obs.Project
}
