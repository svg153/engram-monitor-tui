package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/svg153/engram-monitor-tui/internal/model"
)

type noopService struct{}

func (noopService) Health(context.Context) (model.Health, error) { return model.Health{}, nil }
func (noopService) Stats(context.Context) (model.Stats, error)   { return model.Stats{}, nil }
func (noopService) Search(context.Context, model.SearchParams) ([]model.Observation, error) {
	return nil, nil
}
func (noopService) AllObservations(context.Context) ([]model.Observation, error) { return nil, nil }
func (noopService) RecentSessions(context.Context, int) ([]model.SessionSummary, error) {
	return nil, nil
}
func (noopService) RecentPrompts(context.Context, int) ([]model.Prompt, error) { return nil, nil }
func (noopService) Timeline(context.Context, int64, int, int) (model.TimelineResult, error) {
	return model.TimelineResult{}, nil
}
func (noopService) UpdateObservation(context.Context, int64, model.ObservationUpdate) (model.Observation, error) {
	return model.Observation{}, nil
}
func (noopService) DeletePrompt(context.Context, int64) error      { return nil }
func (noopService) DeleteSession(context.Context, string) error    { return nil }
func (noopService) Export(context.Context, string) ([]byte, error) { return nil, nil }
func (noopService) Import(context.Context, []byte) error           { return nil }
func (noopService) ImportFile(context.Context, string) (model.ExportData, error) {
	return model.ExportData{}, nil
}
func (noopService) MergeProjects(context.Context, string, string) error { return nil }

func seedModel() Model {
	project := "alpha"
	topic := "topic/key"
	m := New(noopService{})
	m.loading = false
	m.data = model.Dataset{
		Health: model.Health{Service: "engram", Status: "ok"},
		Stats:  model.Stats{TotalSessions: 1, TotalObservations: 2, TotalPrompts: 1},
		Observations: []model.Observation{
			{ID: 2, SessionID: "agent-20260521-1", Type: "decision", Title: "second", Content: "beta", Project: &project, Scope: "project", TopicKey: &topic, CreatedAt: "2026-05-21T10:00:00Z", UpdatedAt: "2026-05-21T10:00:00Z"},
			{ID: 1, SessionID: "agent-20260521-1", Type: "bugfix", Title: "first", Content: "alpha", Project: &project, Scope: "personal", CreatedAt: "2026-05-20T10:00:00Z", UpdatedAt: "2026-05-20T10:00:00Z"},
		},
		Sessions: []model.DerivedSession{
			{SessionID: "agent-20260521-1", AgentName: "agent", Project: "alpha", Date: "2026-05-21T10:00:00Z", ObservationCount: 2, LatestTitle: "second", Types: []string{"decision", "bugfix"}, TopicKey: "topic/key"},
		},
		Prompts:       []model.Prompt{{ID: 5, SessionID: "agent-20260521-1", Project: "alpha", Content: "prompt content", CreatedAt: "2026-05-21T10:00:00Z"}},
		EmptySessions: []model.SessionSummary{{ID: "empty-1", Project: "alpha", StartedAt: "2026-05-21T11:00:00Z", ObservationCount: 0}},
		Topics:        []model.TopicGroup{{Key: "topic/key", Projects: []string{"alpha"}, Types: []string{"decision"}, LatestDate: "2026-05-21T10:00:00Z", SearchText: "topic key second", Observations: []model.Observation{{ID: 2, SessionID: "agent-20260521-1", Type: "decision", Title: "second", Content: "beta", Project: &project, Scope: "project", TopicKey: &topic, CreatedAt: "2026-05-21T10:00:00Z", UpdatedAt: "2026-05-21T10:00:00Z"}}}},
		TimelineDays:  []model.TimelineDay{{Key: "2026-05-21", Label: "Today", Observations: []model.Observation{{ID: 2, SessionID: "agent-20260521-1", Type: "decision", Title: "second", Content: "beta", Project: &project, Scope: "project", TopicKey: &topic, CreatedAt: "2026-05-21T10:00:00Z", UpdatedAt: "2026-05-21T10:00:00Z"}}}},
		Projects:      []string{"alpha"},
		Types:         []string{"bugfix", "decision"},
	}
	m.width = 120
	m.height = 40
	m.refreshDetail()
	return m
}

func TestListItemsMemoriesFiltersByScopeAndType(t *testing.T) {
	m := seedModel()
	m.activeTab = TabMemories
	m.typeFilter = 2
	m.scopeFilter = 1

	items := m.listItems()
	if len(items) != 1 {
		t.Fatalf("expected one filtered memory, got %d", len(items))
	}
	if items[0].Title != "second" {
		t.Fatalf("unexpected memory selected: %#v", items[0])
	}
}

func TestDetailContentForPrompt(t *testing.T) {
	m := seedModel()
	m.activeTab = TabPrompts

	content := m.detailContent()
	if content == "" {
		t.Fatalf("expected prompt detail content")
	}
	if got := content; !(containsText(got, "prompt content") && containsText(got, "Prompt #5")) {
		t.Fatalf("unexpected prompt detail: %s", got)
	}
}

func TestOpenEditSwitchesFocus(t *testing.T) {
	m := seedModel()
	m.activeTab = TabMemories
	item := m.selectedItem()
	if item == nil {
		t.Fatal("expected selected item")
	}
	m.openEdit(*item)
	if m.focus != FocusEdit {
		t.Fatalf("expected focus edit, got %v", m.focus)
	}
	if m.edit == nil || m.edit.Original.ID != 2 {
		t.Fatalf("expected edit state for selected observation")
	}
}

func TestSwitchTabResetsContext(t *testing.T) {
	m := seedModel()
	m.currentSession = "agent-20260521-1"
	m.switchTab(1)
	if m.currentSession != "" {
		t.Fatalf("expected session context to reset")
	}
	if m.activeTab != TabSessions {
		t.Fatalf("expected next tab to be sessions, got %s", m.activeTab)
	}
}

func containsText(value, needle string) bool {
	return strings.Contains(value, needle)
}

func TestRenderHelpersProduceOutput(t *testing.T) {
	m := seedModel()
	m.activeTab = TabDashboard

	if got := m.renderHeader(); !containsText(got, "engram-monitor-tui") {
		t.Fatalf("unexpected header: %s", got)
	}
	if got := m.renderSidebar(); !containsText(got, "Dashboard") {
		t.Fatalf("unexpected sidebar: %s", got)
	}
	if got := m.renderFooter(); !containsText(got, "tab focus") {
		t.Fatalf("unexpected footer: %s", got)
	}
}

func TestListItemsAcrossTabs(t *testing.T) {
	m := seedModel()

	m.activeTab = TabSessions
	if items := m.listItems(); len(items) != 1 || items[0].Kind != "session" {
		t.Fatalf("unexpected sessions list: %#v", items)
	}

	m.activeTab = TabTopics
	if items := m.listItems(); len(items) != 1 || items[0].Kind != "topic" {
		t.Fatalf("unexpected topics list: %#v", items)
	}

	m.activeTab = TabTimeline
	if items := m.listItems(); len(items) != 1 || items[0].Kind != "observation" {
		t.Fatalf("unexpected timeline list: %#v", items)
	}

	m.activeTab = TabEmpty
	if items := m.listItems(); len(items) != 1 || items[0].Kind != "empty-session" {
		t.Fatalf("unexpected empty sessions list: %#v", items)
	}
}

func TestOpenModals(t *testing.T) {
	m := seedModel()
	m.openExportModal()
	if m.modal == nil || m.modal.Kind != modalExport {
		t.Fatalf("expected export modal")
	}
	m.openImportModal()
	if m.modal == nil || m.modal.Kind != modalImport {
		t.Fatalf("expected import modal")
	}
	m.openMergeModal()
	if m.modal == nil || m.modal.Kind != modalMerge {
		t.Fatalf("expected merge modal")
	}
}

func TestUpdateSearchAndListNavigation(t *testing.T) {
	m := seedModel()
	m.activeTab = TabMemories
	m.focus = FocusSearch
	m.search.Focus()
	modelOut, _ := m.updateSearch(key("a"))
	m = modelOut.(Model)
	if m.search.Value() == "" {
		t.Fatalf("expected search input to update")
	}

	m.focus = FocusList
	modelOut, _ = m.updateList(tea.KeyMsg{Type: tea.KeyDown})
	m = modelOut.(Model)
	if m.cursor == 0 {
		t.Fatalf("expected cursor to move down")
	}
}

func TestDetailContentVariants(t *testing.T) {
	m := seedModel()
	m.activeTab = TabDashboard
	m.data.Sessions = nil
	if got := m.detailContent(); !containsText(got, "Engram Monitor TUI") {
		t.Fatalf("unexpected dashboard detail: %s", got)
	}

	m = seedModel()
	m.activeTab = TabSessions
	if got := m.detailContent(); !containsText(got, "Press enter to open this session") {
		t.Fatalf("unexpected session detail: %s", got)
	}

	m = seedModel()
	m.activeTab = TabTopics
	if got := m.detailContent(); !containsText(got, "topic/key") {
		t.Fatalf("unexpected topic detail: %s", got)
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
