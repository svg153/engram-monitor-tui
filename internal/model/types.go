package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Observation struct {
	ID             int64   `json:"id"`
	SyncID         string  `json:"sync_id"`
	SessionID      string  `json:"session_id"`
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	Content        string  `json:"content"`
	ToolName       *string `json:"tool_name,omitempty"`
	Project        *string `json:"project,omitempty"`
	Scope          string  `json:"scope"`
	TopicKey       *string `json:"topic_key,omitempty"`
	RevisionCount  int     `json:"revision_count"`
	DuplicateCount int     `json:"duplicate_count"`
	LastSeenAt     *string `json:"last_seen_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	DeletedAt      *string `json:"deleted_at,omitempty"`
	Rank           float64 `json:"rank,omitempty"`
}

type ObservationUpdate struct {
	Title    *string `json:"title,omitempty"`
	Content  *string `json:"content,omitempty"`
	Type     *string `json:"type,omitempty"`
	Scope    *string `json:"scope,omitempty"`
	TopicKey *string `json:"topic_key,omitempty"`
}

type SessionSummary struct {
	ID               string  `json:"id"`
	Project          string  `json:"project"`
	StartedAt        string  `json:"started_at"`
	EndedAt          *string `json:"ended_at,omitempty"`
	Summary          *string `json:"summary,omitempty"`
	ObservationCount int     `json:"observation_count"`
}

type Prompt struct {
	ID        int64  `json:"id"`
	SyncID    string `json:"sync_id"`
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	Project   string `json:"project,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Stats struct {
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects"`
}

type Health struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type TimelineEntry struct {
	ID             int64   `json:"id"`
	SessionID      string  `json:"session_id"`
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	Content        string  `json:"content"`
	ToolName       *string `json:"tool_name,omitempty"`
	Project        *string `json:"project,omitempty"`
	Scope          string  `json:"scope"`
	TopicKey       *string `json:"topic_key,omitempty"`
	RevisionCount  int     `json:"revision_count"`
	DuplicateCount int     `json:"duplicate_count"`
	LastSeenAt     *string `json:"last_seen_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	DeletedAt      *string `json:"deleted_at,omitempty"`
	IsFocus        bool    `json:"is_focus"`
}

type Session struct {
	ID        string  `json:"id"`
	Project   string  `json:"project"`
	Directory string  `json:"directory"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at,omitempty"`
	Summary   *string `json:"summary,omitempty"`
}

type TimelineResult struct {
	Focus        Observation     `json:"focus"`
	Before       []TimelineEntry `json:"before"`
	After        []TimelineEntry `json:"after"`
	SessionInfo  *Session        `json:"session_info"`
	TotalInRange int             `json:"total_in_range"`
}

type ExportData struct {
	Version      string        `json:"version"`
	ExportedAt   string        `json:"exported_at"`
	Sessions     []Session     `json:"sessions"`
	Observations []Observation `json:"observations"`
	Prompts      []Prompt      `json:"prompts"`
}

type SearchParams struct {
	Q       string
	Type    string
	Project string
	Scope   string
	Limit   int
}

type DerivedSession struct {
	SessionID         string
	AgentName         string
	Project           string
	Date              string
	ObservationCount  int
	LatestTitle       string
	Types             []string
	TopicKey          string
	Observations      []Observation
	SessionSummaryRef *SessionSummary
}

type TopicGroup struct {
	Key          string
	Observations []Observation
	Projects     []string
	Types        []string
	LatestDate   string
	SearchText   string
}

type TimelineDay struct {
	Key          string
	Label        string
	Observations []Observation
}

type Dataset struct {
	Health           Health
	Stats            Stats
	Observations     []Observation
	Sessions         []DerivedSession
	SessionSummaries []SessionSummary
	Prompts          []Prompt
	EmptySessions    []SessionSummary
	Topics           []TopicGroup
	TimelineDays     []TimelineDay
	Projects         []string
	Types            []string
}

func DeriveDataset(health Health, stats Stats, observations []Observation, sessionSummaries []SessionSummary, prompts []Prompt) Dataset {
	sessions := DeriveSessions(observations, sessionSummaries)
	projects := deriveProjects(stats.Projects, observations, sessionSummaries, prompts)
	return Dataset{
		Health:           health,
		Stats:            stats,
		Observations:     sortObservationsDesc(observations),
		Sessions:         sessions,
		SessionSummaries: sessionSummaries,
		Prompts:          sortPromptsDesc(prompts),
		EmptySessions:    filterEmptySessions(sessionSummaries),
		Topics:           GroupTopics(observations),
		TimelineDays:     GroupTimeline(observations),
		Projects:         projects,
		Types:            deriveTypes(observations),
	}
}

func DeriveSessions(observations []Observation, summaries []SessionSummary) []DerivedSession {
	grouped := make(map[string][]Observation)
	summaryByID := make(map[string]SessionSummary, len(summaries))
	for _, summary := range summaries {
		summaryByID[summary.ID] = summary
	}
	for _, obs := range observations {
		grouped[obs.SessionID] = append(grouped[obs.SessionID], obs)
	}
	out := make([]DerivedSession, 0, len(grouped))
	for sessionID, obs := range grouped {
		sorted := sortObservationsAsc(obs)
		last := sorted[len(sorted)-1]
		project := projectOf(last)
		if summary, ok := summaryByID[sessionID]; ok && summary.Project != "" {
			project = summary.Project
		}
		typesSet := make(map[string]struct{})
		var types []string
		topicKey := ""
		for _, item := range sorted {
			if _, ok := typesSet[item.Type]; !ok {
				typesSet[item.Type] = struct{}{}
				types = append(types, item.Type)
			}
			if topicKey == "" && item.TopicKey != nil {
				topicKey = *item.TopicKey
			}
		}
		session := DerivedSession{
			SessionID:        sessionID,
			AgentName:        deriveAgentName(sessionID),
			Project:          project,
			Date:             last.CreatedAt,
			ObservationCount: len(sorted),
			LatestTitle:      last.Title,
			Types:            types,
			TopicKey:         topicKey,
			Observations:     sortObservationsDesc(sorted),
		}
		if summary, ok := summaryByID[sessionID]; ok {
			summaryCopy := summary
			session.SessionSummaryRef = &summaryCopy
		}
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

func GroupTopics(observations []Observation) []TopicGroup {
	grouped := make(map[string][]Observation)
	for _, obs := range observations {
		if obs.TopicKey == nil || strings.TrimSpace(*obs.TopicKey) == "" {
			continue
		}
		grouped[*obs.TopicKey] = append(grouped[*obs.TopicKey], obs)
	}
	out := make([]TopicGroup, 0, len(grouped))
	for key, obs := range grouped {
		sorted := sortObservationsAsc(obs)
		projectSet := make(map[string]struct{})
		typeSet := make(map[string]struct{})
		var projects []string
		var types []string
		var searchable []string
		searchable = append(searchable, key)
		for _, item := range sorted {
			project := projectOf(item)
			if project != "" {
				if _, ok := projectSet[project]; !ok {
					projectSet[project] = struct{}{}
					projects = append(projects, project)
				}
			}
			if _, ok := typeSet[item.Type]; !ok {
				typeSet[item.Type] = struct{}{}
				types = append(types, item.Type)
			}
			searchable = append(searchable, item.Title, item.Content)
		}
		out = append(out, TopicGroup{
			Key:          key,
			Observations: sortObservationsDesc(sorted),
			Projects:     projects,
			Types:        types,
			LatestDate:   sorted[len(sorted)-1].CreatedAt,
			SearchText:   strings.ToLower(strings.Join(searchable, " ")),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LatestDate > out[j].LatestDate })
	return out
}

func GroupTimeline(observations []Observation) []TimelineDay {
	grouped := make(map[string][]Observation)
	sorted := sortObservationsDesc(observations)
	for _, obs := range sorted {
		key := dateKey(obs.CreatedAt)
		grouped[key] = append(grouped[key], obs)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	out := make([]TimelineDay, 0, len(keys))
	for _, key := range keys {
		out = append(out, TimelineDay{
			Key:          key,
			Label:        humanDayLabel(key),
			Observations: grouped[key],
		})
	}
	return out
}

func dateKey(iso string) string {
	if len(iso) >= 10 {
		if t, err := time.Parse(time.RFC3339, iso); err == nil {
			return t.Local().Format("2006-01-02")
		}
	}
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}

func humanDayLabel(key string) string {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
	switch key {
	case today:
		return "Today"
	case yesterday:
		return "Yesterday"
	default:
		if t, err := time.Parse("2006-01-02", key); err == nil {
			return t.Format("Mon Jan 2")
		}
		return key
	}
}

func sortObservationsDesc(in []Observation) []Observation {
	out := append([]Observation(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func sortObservationsAsc(in []Observation) []Observation {
	out := append([]Observation(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

func sortPromptsDesc(in []Prompt) []Prompt {
	out := append([]Prompt(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func filterEmptySessions(in []SessionSummary) []SessionSummary {
	var out []SessionSummary
	for _, item := range in {
		if item.ObservationCount == 0 {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

func deriveProjects(base []string, observations []Observation, summaries []SessionSummary, prompts []Prompt) []string {
	set := make(map[string]struct{})
	for _, project := range base {
		if strings.TrimSpace(project) != "" {
			set[project] = struct{}{}
		}
	}
	for _, obs := range observations {
		if project := projectOf(obs); project != "" {
			set[project] = struct{}{}
		}
	}
	for _, summary := range summaries {
		if strings.TrimSpace(summary.Project) != "" {
			set[summary.Project] = struct{}{}
		}
	}
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt.Project) != "" {
			set[prompt.Project] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for project := range set {
		out = append(out, project)
	}
	sort.Strings(out)
	return out
}

func deriveTypes(observations []Observation) []string {
	set := make(map[string]struct{})
	for _, obs := range observations {
		if strings.TrimSpace(obs.Type) != "" {
			set[obs.Type] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for typ := range set {
		out = append(out, typ)
	}
	sort.Strings(out)
	return out
}

func deriveAgentName(sessionID string) string {
	if idx := strings.Index(sessionID, "-20"); idx > 0 {
		return sessionID[:idx]
	}
	if strings.HasPrefix(sessionID, "manual-save-") {
		return "manual"
	}
	return sessionID
}

func projectOf(obs Observation) string {
	if obs.Project == nil {
		return ""
	}
	return strings.TrimSpace(*obs.Project)
}

func PrettySince(iso string) string {
	if iso == "" {
		return "-"
	}
	ts, err := parseTime(iso)
	if err != nil {
		return iso
	}
	diff := time.Since(ts)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return ts.Format("2006-01-02")
	}
}

func PrettyTime(iso string) string {
	ts, err := parseTime(iso)
	if err != nil {
		return iso
	}
	return ts.Local().Format("2006-01-02 15:04")
}

func parseTime(iso string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if ts, err := time.Parse(layout, iso); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp: %s", iso)
}
