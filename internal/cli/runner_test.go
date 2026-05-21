package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/svg153/engram-monitor-tui/internal/api"
	"github.com/svg153/engram-monitor-tui/internal/model"
)

type fakeService struct {
	health       model.Health
	stats        model.Stats
	observations []model.Observation
	sessions     []model.SessionSummary
	prompts      []model.Prompt
	searchResult []model.Observation
	exportData   []byte
	mergedFrom   string
	mergedTo     string
}

var _ api.Service = (*fakeService)(nil)

func (f *fakeService) Health(context.Context) (model.Health, error) { return f.health, nil }
func (f *fakeService) Stats(context.Context) (model.Stats, error)   { return f.stats, nil }
func (f *fakeService) Search(context.Context, model.SearchParams) ([]model.Observation, error) {
	return f.searchResult, nil
}
func (f *fakeService) AllObservations(context.Context) ([]model.Observation, error) {
	return f.observations, nil
}
func (f *fakeService) RecentSessions(context.Context, int) ([]model.SessionSummary, error) {
	return f.sessions, nil
}
func (f *fakeService) RecentPrompts(context.Context, int) ([]model.Prompt, error) {
	return f.prompts, nil
}
func (f *fakeService) Timeline(context.Context, int64, int, int) (model.TimelineResult, error) {
	return model.TimelineResult{}, nil
}
func (f *fakeService) UpdateObservation(context.Context, int64, model.ObservationUpdate) (model.Observation, error) {
	return model.Observation{}, nil
}
func (f *fakeService) DeletePrompt(context.Context, int64) error      { return nil }
func (f *fakeService) DeleteSession(context.Context, string) error    { return nil }
func (f *fakeService) Export(context.Context, string) ([]byte, error) { return f.exportData, nil }
func (f *fakeService) Import(context.Context, []byte) error           { return nil }
func (f *fakeService) ImportFile(context.Context, string) (model.ExportData, error) {
	return model.ExportData{}, nil
}
func (f *fakeService) MergeProjects(_ context.Context, from, to string) error {
	f.mergedFrom = from
	f.mergedTo = to
	return nil
}

func testRunner(service *fakeService) Runner {
	return Runner{
		Version: "test",
		NewService: func(string) api.Service {
			return service
		},
		RunTUI: func(api.Service) error { return nil },
	}
}

func TestProjectsListText(t *testing.T) {
	service := &fakeService{stats: model.Stats{Projects: []string{"alpha", "beta"}}}
	runner := testRunner(service)
	var stdout, stderr bytes.Buffer

	code := runner.Run([]string{"projects", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestMemoriesSearchJSON(t *testing.T) {
	project := "alpha"
	service := &fakeService{searchResult: []model.Observation{{ID: 7, Type: "decision", Title: "hello", Project: &project, SessionID: "s1"}}}
	runner := testRunner(service)
	var stdout, stderr bytes.Buffer

	code := runner.Run([]string{"memories", "search", "--query", "hello", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "\"hello\"") || !strings.Contains(got, "\"decision\"") {
		t.Fatalf("unexpected JSON output: %s", got)
	}
}

func TestSessionsListFiltersProject(t *testing.T) {
	project := "alpha"
	service := &fakeService{
		observations: []model.Observation{
			{ID: 1, SessionID: "agent-20260521-1", Type: "decision", Title: "title", Content: "content", Project: &project, Scope: "project", CreatedAt: "2026-05-21T10:00:00Z"},
		},
		sessions: []model.SessionSummary{{ID: "agent-20260521-1", Project: "alpha", ObservationCount: 1}},
	}
	runner := testRunner(service)
	var stdout, stderr bytes.Buffer

	code := runner.Run([]string{"sessions", "list", "--project", "alpha"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "agent-20260521-1") {
		t.Fatalf("expected session output, got %s", got)
	}
}

func TestExportWritesFile(t *testing.T) {
	service := &fakeService{exportData: []byte(`{"version":"1"}`)}
	runner := testRunner(service)
	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	outPath := filepath.Join(dir, "export.json")

	code := runner.Run([]string{"export", "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if string(raw) != `{"version":"1"}` {
		t.Fatalf("unexpected export content: %s", string(raw))
	}
}

func TestMergeProjects(t *testing.T) {
	service := &fakeService{}
	runner := testRunner(service)
	var stdout, stderr bytes.Buffer

	code := runner.Run([]string{"merge-projects", "--from", "old", "--to", "new"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if service.mergedFrom != "old" || service.mergedTo != "new" {
		t.Fatalf("expected merge arguments to be forwarded")
	}
}
