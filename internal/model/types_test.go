package model

import (
	"strings"
	"testing"
)

func TestDeriveSessionsBuildsAndSorts(t *testing.T) {
	project := "alpha"
	topic := "memory/topic"
	summaries := []SessionSummary{
		{ID: "agent-20260521-1", Project: "alpha", ObservationCount: 2},
	}
	observations := []Observation{
		{ID: 1, SessionID: "agent-20260521-1", Type: "decision", Title: "older", Content: "old", Project: &project, Scope: "project", CreatedAt: "2026-05-20T10:00:00Z"},
		{ID: 2, SessionID: "agent-20260521-1", Type: "architecture", Title: "newer", Content: "new", Project: &project, Scope: "project", TopicKey: &topic, CreatedAt: "2026-05-21T10:00:00Z"},
	}

	sessions := DeriveSessions(observations, summaries)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	got := sessions[0]
	if got.AgentName != "agent" {
		t.Fatalf("expected agent name derived from session id, got %q", got.AgentName)
	}
	if got.LatestTitle != "newer" {
		t.Fatalf("expected latest title newer, got %q", got.LatestTitle)
	}
	if got.TopicKey != topic {
		t.Fatalf("expected topic key %q, got %q", topic, got.TopicKey)
	}
	if len(got.Types) != 2 {
		t.Fatalf("expected 2 unique types, got %d", len(got.Types))
	}
	if got.SessionSummaryRef == nil || got.SessionSummaryRef.Project != "alpha" {
		t.Fatalf("expected session summary attached")
	}
}

func TestGroupTopicsAggregatesProjectsAndTypes(t *testing.T) {
	projectA := "alpha"
	projectB := "beta"
	key := "topic/refactor"
	observations := []Observation{
		{ID: 1, SessionID: "s1", Type: "decision", Title: "first", Content: "content", Project: &projectA, Scope: "project", TopicKey: &key, CreatedAt: "2026-05-20T10:00:00Z"},
		{ID: 2, SessionID: "s2", Type: "bugfix", Title: "second", Content: "other", Project: &projectB, Scope: "project", TopicKey: &key, CreatedAt: "2026-05-21T10:00:00Z"},
	}

	topics := GroupTopics(observations)
	if len(topics) != 1 {
		t.Fatalf("expected 1 topic group, got %d", len(topics))
	}
	group := topics[0]
	if group.Key != key {
		t.Fatalf("expected key %q, got %q", key, group.Key)
	}
	if len(group.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(group.Projects))
	}
	if len(group.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(group.Types))
	}
	if !strings.Contains(group.SearchText, "second") {
		t.Fatalf("expected search text to contain observation title")
	}
}

func TestGroupTimelineOrdersDaysNewestFirst(t *testing.T) {
	project := "alpha"
	observations := []Observation{
		{ID: 1, SessionID: "s1", Type: "decision", Title: "today", Content: "a", Project: &project, Scope: "project", CreatedAt: "2026-05-21T10:00:00Z"},
		{ID: 2, SessionID: "s1", Type: "decision", Title: "older", Content: "b", Project: &project, Scope: "project", CreatedAt: "2026-05-20T10:00:00Z"},
	}
	days := GroupTimeline(observations)
	if len(days) != 2 {
		t.Fatalf("expected 2 timeline days, got %d", len(days))
	}
	if days[0].Key != "2026-05-21" {
		t.Fatalf("expected newest day first, got %q", days[0].Key)
	}
	if len(days[0].Observations) != 1 || days[0].Observations[0].ID != 1 {
		t.Fatalf("expected newest observation in first day")
	}
}

func TestDeriveDatasetCollectsProjectsAndEmptySessions(t *testing.T) {
	project := "alpha"
	health := Health{Service: "engram", Status: "ok", Version: "0.1.0"}
	stats := Stats{Projects: []string{"base"}}
	observations := []Observation{{ID: 1, SessionID: "s1", Type: "decision", Title: "t", Content: "c", Project: &project, Scope: "project", CreatedAt: "2026-05-21T10:00:00Z"}}
	summaries := []SessionSummary{
		{ID: "s1", Project: "alpha", ObservationCount: 1},
		{ID: "empty-1", Project: "beta", ObservationCount: 0},
	}
	prompts := []Prompt{{ID: 1, Project: "gamma", CreatedAt: "2026-05-21T10:00:00Z"}}

	data := DeriveDataset(health, stats, observations, summaries, prompts)
	if len(data.EmptySessions) != 1 || data.EmptySessions[0].ID != "empty-1" {
		t.Fatalf("expected one empty session")
	}
	if !containsString(data.Projects, "base") || !containsString(data.Projects, "alpha") || !containsString(data.Projects, "gamma") {
		t.Fatalf("expected derived projects to include stats, observations, and prompts: %#v", data.Projects)
	}
}

func TestPrettyTimeAndSince(t *testing.T) {
	if got := PrettyTime("2026-05-21T10:00:00Z"); !strings.Contains(got, "2026-05-21") {
		t.Fatalf("unexpected pretty time: %q", got)
	}
	if got := PrettySince("2026-05-21T10:00:00Z"); got == "" {
		t.Fatal("expected pretty since output")
	}
	if _, err := parseTime("2026-05-21T10:00:00Z"); err != nil {
		t.Fatalf("parseTime returned error: %v", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
