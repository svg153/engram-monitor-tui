package app

import (
	"testing"

	"github.com/svg153/engram-monitor-tui/internal/model"
)

func TestPrettyTab(t *testing.T) {
	tests := []struct {
		tab    Tab
		expect string
	}{
		{TabDashboard, "Dashboard"},
		{TabSessions, "Sessions"},
		{TabMemories, "Memories"},
		{TabTopics, "Topics"},
		{TabTimeline, "Timeline"},
		{TabPrompts, "Prompts"},
		{TabEmpty, "Empty Sessions"},
		{Tab("unknown"), "unknown"},
	}
	for _, tt := range tests {
		got := prettyTab(tt.tab)
		if got != tt.expect {
			t.Errorf("prettyTab(%q) = %q, want %q", tt.tab, got, tt.expect)
		}
	}
}

func TestIndexOfTab(t *testing.T) {
	if got := indexOfTab(TabDashboard); got != 0 {
		t.Errorf("indexOfTab(TabDashboard) = %d, want 0", got)
	}
	if got := indexOfTab(TabSessions); got != 1 {
		t.Errorf("indexOfTab(TabSessions) = %d, want 1", got)
	}
	if got := indexOfTab(TabTimeline); got != 4 {
		t.Errorf("indexOfTab(TabTimeline) = %d, want 4", got)
	}
	if got := indexOfTab(TabEmpty); got != 6 {
		t.Errorf("indexOfTab(TabEmpty) = %d, want 6", got)
	}
	// Tab not in orderedTabs returns 0
	if got := indexOfTab(Tab("fake")); got != 0 {
		t.Errorf("indexOfTab(Tab('fake')) = %d, want 0", got)
	}
}

func TestFocusLabel(t *testing.T) {
	m := Model{}
	tests := []struct {
		focus  Focus
		expect string
	}{
		{FocusSidebar, "sidebar"},
		{FocusList, "list"},
		{FocusDetail, "detail"},
		{FocusSearch, "search"},
		{FocusEdit, "edit"},
		{FocusModal, "modal"},
		{Focus(99), "unknown"},
	}
	for _, tt := range tests {
		m.focus = tt.focus
		got := m.focusLabel()
		if got != tt.expect {
			t.Errorf("focusLabel(%d) = %q, want %q", tt.focus, got, tt.expect)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		limit  int
		expect string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello…"},
		{"", 5, ""},
		{"a", 1, "a"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.limit)
		if got != tt.expect {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.limit, got, tt.expect)
		}
	}
}

func TestOrFallback(t *testing.T) {
	tests := []struct {
		input    string
		fallback string
		expect   string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"  ", "default", "default"},
		{"\t\n", "default", "default"},
	}
	for _, tt := range tests {
		got := orFallback(tt.input, tt.fallback)
		if got != tt.expect {
			t.Errorf("orFallback(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.expect)
		}
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"a", "b", "c"}, "b") {
		t.Error("contains should return true for existing element")
	}
	if contains([]string{"a", "b", "c"}, "d") {
		t.Error("contains should return false for missing element")
	}
	if contains([]string{}, "a") {
		t.Error("contains on empty slice should return false")
	}
}

func TestTopicValue(t *testing.T) {
	s := "test-topic"
	if got := topicValue(&s); got != "test-topic" {
		t.Errorf("topicValue(&s) = %q, want %q", got, "test-topic")
	}
	if got := topicValue(nil); got != "" {
		t.Errorf("topicValue(nil) = %q, want empty", got)
	}
}

func TestProjectOf(t *testing.T) {
	proj := "alpha"
	obs := model.Observation{Project: &proj}
	if got := projectOf(obs); got != "alpha" {
		t.Errorf("projectOf(obs) = %q, want %q", got, "alpha")
	}
	obs2 := model.Observation{}
	if got := projectOf(obs2); got != "" {
		t.Errorf("projectOf(obs with nil Project) = %q, want empty", got)
	}
}

func TestMin(t *testing.T) {
	if got := min(3, 5); got != 3 {
		t.Errorf("min(3, 5) = %d, want 3", got)
	}
	if got := min(5, 3); got != 3 {
		t.Errorf("min(5, 3) = %d, want 3", got)
	}
	if got := min(3, 3); got != 3 {
		t.Errorf("min(3, 3) = %d, want 3", got)
	}
}

func TestMax(t *testing.T) {
	if got := max(3, 5); got != 5 {
		t.Errorf("max(3, 5) = %d, want 5", got)
	}
	if got := max(5, 3); got != 5 {
		t.Errorf("max(5, 3) = %d, want 5", got)
	}
	if got := max(3, 3); got != 3 {
		t.Errorf("max(3, 3) = %d, want 3", got)
	}
}

func TestPtr(t *testing.T) {
	v := ptr(42)
	if *v != 42 {
		t.Errorf("ptr(42) = %d, want 42", *v)
	}
	s := ptr("hello")
	if *s != "hello" {
		t.Errorf("ptr(\"hello\") = %q, want %q", *s, "hello")
	}
}

func TestJsonUnmarshal(t *testing.T) {
	var m map[string]any
	err := jsonUnmarshal([]byte(`{"key":"value"}`), &m)
	if err != nil {
		t.Fatalf("jsonUnmarshal should not error: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("jsonUnmarshal parsed value: got %v, want value", m["key"])
	}
}
