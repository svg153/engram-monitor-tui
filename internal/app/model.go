package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/svg153/engram-monitor-tui/internal/api"
	"github.com/svg153/engram-monitor-tui/internal/model"
)

type Tab string

const (
	TabDashboard Tab = "dashboard"
	TabSessions  Tab = "sessions"
	TabMemories  Tab = "memories"
	TabTopics    Tab = "topics"
	TabTimeline  Tab = "timeline"
	TabPrompts   Tab = "prompts"
	TabEmpty     Tab = "empty"
)

var orderedTabs = []Tab{
	TabDashboard,
	TabSessions,
	TabMemories,
	TabTopics,
	TabTimeline,
	TabPrompts,
	TabEmpty,
}

type Focus int

const (
	FocusSidebar Focus = iota
	FocusList
	FocusDetail
	FocusSearch
	FocusEdit
	FocusModal
)

type listItem struct {
	Title    string
	Subtitle string
	Meta     string
	Kind     string
	Value    any
}

type modalKind string

const (
	modalExport  modalKind = "export"
	modalImport  modalKind = "import"
	modalMerge   modalKind = "merge"
	modalConfirm modalKind = "confirm"
)

type modalState struct {
	Kind         modalKind
	Title        string
	Description  string
	Inputs       []textinput.Model
	Active       int
	ConfirmLabel string
	Danger       bool
	Payload      any
}

type editState struct {
	Original model.Observation
	Fields   []textinput.Model
	Content  textarea.Model
	Active   int
}

type loadMsg struct {
	data model.Dataset
	err  error
}

type actionDoneMsg struct {
	status string
	err    error
	reload bool
}

type timelineMsg struct {
	result model.TimelineResult
	err    error
}

type Model struct {
	client         api.Service
	width          int
	height         int
	activeTab      Tab
	focus          Focus
	search         textinput.Model
	projectFilter  int
	typeFilter     int
	scopeFilter    int
	cursor         int
	scroll         int
	detail         viewport.Model
	loading        bool
	spin           spinner.Model
	data           model.Dataset
	status         string
	err            string
	lastLoaded     time.Time
	currentSession string
	currentTopic   string
	edit           *editState
	modal          *modalState
	timeline       *model.TimelineResult
}

func New(client api.Service) Model {
	search := textinput.New()
	search.Placeholder = "Search current view…"
	search.Prompt = "Search > "
	search.CharLimit = 256
	search.Width = 32

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	detail := viewport.New(40, 20)

	return Model{
		client:    client,
		activeTab: TabDashboard,
		focus:     FocusList,
		search:    search,
		detail:    detail,
		loading:   true,
		spin:      spin,
		status:    "Connecting to Engram…",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadAll())
}

func (m Model) loadAll() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		type result[T any] struct {
			value T
			err   error
		}

		healthCh := make(chan result[model.Health], 1)
		statsCh := make(chan result[model.Stats], 1)
		obsCh := make(chan result[[]model.Observation], 1)
		sessionsCh := make(chan result[[]model.SessionSummary], 1)
		promptsCh := make(chan result[[]model.Prompt], 1)

		go func() {
			value, err := m.client.Health(ctx)
			healthCh <- result[model.Health]{value: value, err: err}
		}()
		go func() {
			value, err := m.client.Stats(ctx)
			statsCh <- result[model.Stats]{value: value, err: err}
		}()
		go func() {
			value, err := m.client.AllObservations(ctx)
			obsCh <- result[[]model.Observation]{value: value, err: err}
		}()
		go func() {
			value, err := m.client.RecentSessions(ctx, 1000)
			sessionsCh <- result[[]model.SessionSummary]{value: value, err: err}
		}()
		go func() {
			value, err := m.client.RecentPrompts(ctx, 500)
			promptsCh <- result[[]model.Prompt]{value: value, err: err}
		}()

		healthRes := <-healthCh
		statsRes := <-statsCh
		obsRes := <-obsCh
		sessionsRes := <-sessionsCh
		promptsRes := <-promptsCh

		var errs []string
		for _, err := range []error{healthRes.err, statsRes.err, obsRes.err, sessionsRes.err, promptsRes.err} {
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			return loadMsg{err: errors.New(strings.Join(errs, "; "))}
		}

		return loadMsg{
			data: model.DeriveDataset(
				healthRes.value,
				statsRes.value,
				obsRes.value,
				sessionsRes.value,
				promptsRes.value,
			),
		}
	}
}

func (m *Model) listItems() []listItem {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	project := m.projectFilterValue()
	typ := m.typeFilterValue()
	scope := m.scopeFilterValue()

	switch m.activeTab {
	case TabDashboard:
		items := make([]listItem, 0, min(len(m.data.Sessions), 12))
		for _, session := range m.data.Sessions {
			if len(items) >= 12 {
				break
			}
			items = append(items, listItem{
				Title:    session.LatestTitle,
				Subtitle: fmt.Sprintf("%s · %s", session.Project, session.AgentName),
				Meta:     fmt.Sprintf("%s · %d obs", model.PrettySince(session.Date), session.ObservationCount),
				Kind:     "session",
				Value:    session,
			})
		}
		return items
	case TabSessions:
		if m.currentSession != "" {
			session := m.findSession(m.currentSession)
			if session == nil {
				return nil
			}
			var items []listItem
			for _, obs := range session.Observations {
				if query != "" && !strings.Contains(strings.ToLower(obs.Title+" "+obs.Content+" "+topicValue(obs.TopicKey)), query) {
					continue
				}
				if typ != "" && obs.Type != typ {
					continue
				}
				items = append(items, observationItem(obs))
			}
			return items
		}
		var items []listItem
		for _, session := range m.data.Sessions {
			if project != "" && session.Project != project {
				continue
			}
			if typ != "" && !contains(session.Types, typ) {
				continue
			}
			searchText := strings.ToLower(strings.Join([]string{session.AgentName, session.Project, session.LatestTitle, session.TopicKey, session.SessionID}, " "))
			if query != "" && !strings.Contains(searchText, query) {
				continue
			}
			items = append(items, listItem{
				Title:    orFallback(session.LatestTitle, "(no title)"),
				Subtitle: fmt.Sprintf("%s · %s", session.Project, session.AgentName),
				Meta:     fmt.Sprintf("%s · %d obs · %s", model.PrettySince(session.Date), session.ObservationCount, truncate(session.SessionID, 30)),
				Kind:     "session",
				Value:    session,
			})
		}
		return items
	case TabMemories:
		var items []listItem
		for _, obs := range m.data.Observations {
			if project != "" && projectOf(obs) != project {
				continue
			}
			if typ != "" && obs.Type != typ {
				continue
			}
			if scope != "" && obs.Scope != scope {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(obs.Title+" "+obs.Content+" "+topicValue(obs.TopicKey)), query) {
				continue
			}
			items = append(items, observationItem(obs))
		}
		return items
	case TabTopics:
		if m.currentTopic != "" {
			topic := m.findTopic(m.currentTopic)
			if topic == nil {
				return nil
			}
			var items []listItem
			for _, obs := range topic.Observations {
				if query != "" && !strings.Contains(strings.ToLower(obs.Title+" "+obs.Content), query) {
					continue
				}
				items = append(items, observationItem(obs))
			}
			return items
		}
		var items []listItem
		for _, topic := range m.data.Topics {
			if project != "" && !contains(topic.Projects, project) {
				continue
			}
			if query != "" && !strings.Contains(topic.SearchText, query) {
				continue
			}
			items = append(items, listItem{
				Title:    topic.Key,
				Subtitle: strings.Join(topic.Projects, ", "),
				Meta:     fmt.Sprintf("%s · %d obs", model.PrettySince(topic.LatestDate), len(topic.Observations)),
				Kind:     "topic",
				Value:    topic,
			})
		}
		return items
	case TabTimeline:
		var items []listItem
		for _, day := range m.data.TimelineDays {
			for _, obs := range day.Observations {
				if project != "" && projectOf(obs) != project {
					continue
				}
				if typ != "" && obs.Type != typ {
					continue
				}
				if query != "" && !strings.Contains(strings.ToLower(obs.Title+" "+obs.Content+" "+topicValue(obs.TopicKey)), query) {
					continue
				}
				item := observationItem(obs)
				item.Subtitle = fmt.Sprintf("%s · %s", day.Label, item.Subtitle)
				items = append(items, item)
			}
		}
		return items
	case TabPrompts:
		var items []listItem
		for _, prompt := range m.data.Prompts {
			if project != "" && prompt.Project != project {
				continue
			}
			searchText := strings.ToLower(prompt.Content + " " + prompt.Project + " " + prompt.SessionID)
			if query != "" && !strings.Contains(searchText, query) {
				continue
			}
			items = append(items, listItem{
				Title:    truncate(prompt.Content, 60),
				Subtitle: fmt.Sprintf("%s · %s", orFallback(prompt.Project, "(no project)"), truncate(prompt.SessionID, 24)),
				Meta:     model.PrettySince(prompt.CreatedAt),
				Kind:     "prompt",
				Value:    prompt,
			})
		}
		return items
	case TabEmpty:
		var items []listItem
		for _, session := range m.data.EmptySessions {
			if project != "" && session.Project != project {
				continue
			}
			searchText := strings.ToLower(session.ID + " " + session.Project)
			if query != "" && !strings.Contains(searchText, query) {
				continue
			}
			items = append(items, listItem{
				Title:    truncate(session.ID, 48),
				Subtitle: orFallback(session.Project, "(no project)"),
				Meta:     model.PrettySince(session.StartedAt),
				Kind:     "empty-session",
				Value:    session,
			})
		}
		return items
	default:
		return nil
	}
}

func observationItem(obs model.Observation) listItem {
	return listItem{
		Title:    obs.Title,
		Subtitle: fmt.Sprintf("%s · %s · %s", obs.Type, orFallback(projectOf(obs), "(no project)"), orFallback(obs.Scope, "project")),
		Meta:     fmt.Sprintf("%s · #%d", model.PrettySince(obs.CreatedAt), obs.ID),
		Kind:     "observation",
		Value:    obs,
	}
}

func (m *Model) selectedItem() *listItem {
	items := m.listItems()
	if len(items) == 0 {
		m.cursor = 0
		m.scroll = 0
		return nil
	}
	if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	item := items[m.cursor]
	return &item
}

func (m *Model) projectFilterValue() string {
	if m.projectFilter <= 0 || m.projectFilter > len(m.data.Projects) {
		return ""
	}
	return m.data.Projects[m.projectFilter-1]
}

func (m *Model) typeFilterValue() string {
	if m.typeFilter <= 0 || m.typeFilter > len(m.data.Types) {
		return ""
	}
	return m.data.Types[m.typeFilter-1]
}

func (m *Model) scopeFilterValue() string {
	options := []string{"", "project", "personal", "global"}
	if m.scopeFilter < 0 || m.scopeFilter >= len(options) {
		return ""
	}
	return options[m.scopeFilter]
}

func (m *Model) findSession(id string) *model.DerivedSession {
	for _, session := range m.data.Sessions {
		if session.SessionID == id {
			copy := session
			return &copy
		}
	}
	return nil
}

func (m *Model) findTopic(key string) *model.TopicGroup {
	for _, topic := range m.data.Topics {
		if topic.Key == key {
			copy := topic
			return &copy
		}
	}
	return nil
}

func (m *Model) openExportModal() {
	pathInput := textinput.New()
	pathInput.Prompt = "Path: "
	pathInput.SetValue(filepath.Join(".", fmt.Sprintf("engram-export-%s.json", time.Now().Format("20060102-150405"))))
	pathInput.Focus()

	projectInput := textinput.New()
	projectInput.Prompt = "Project (optional): "

	m.modal = &modalState{
		Kind:         modalExport,
		Title:        "Export JSON",
		Description:  "Write an Engram export to a JSON file.",
		Inputs:       []textinput.Model{pathInput, projectInput},
		ConfirmLabel: "Export",
	}
	m.focus = FocusModal
}

func (m *Model) openImportModal() {
	pathInput := textinput.New()
	pathInput.Prompt = "JSON path: "
	pathInput.SetValue("./engram-export.json")
	pathInput.Focus()
	m.modal = &modalState{
		Kind:         modalImport,
		Title:        "Import JSON",
		Description:  "Load an Engram JSON export and import it into the local server.",
		Inputs:       []textinput.Model{pathInput},
		ConfirmLabel: "Validate",
		Danger:       true,
	}
	m.focus = FocusModal
}

func (m *Model) openMergeModal() {
	fromInput := textinput.New()
	fromInput.Prompt = "From project: "
	fromInput.Focus()
	toInput := textinput.New()
	toInput.Prompt = "To project: "
	m.modal = &modalState{
		Kind:         modalMerge,
		Title:        "Merge projects",
		Description:  "Move all observations from one project into another.",
		Inputs:       []textinput.Model{fromInput, toInput},
		ConfirmLabel: "Merge",
		Danger:       true,
	}
	m.focus = FocusModal
}

func (m *Model) openDeleteConfirm(item listItem) {
	var description string
	switch value := item.Value.(type) {
	case model.Prompt:
		description = fmt.Sprintf("Delete prompt #%d? This cannot be undone.", value.ID)
	case model.SessionSummary:
		description = fmt.Sprintf("Delete empty session %q? This cannot be undone.", value.ID)
	default:
		return
	}
	m.modal = &modalState{
		Kind:         modalConfirm,
		Title:        "Confirm destructive action",
		Description:  description,
		ConfirmLabel: "Delete",
		Danger:       true,
		Payload:      item,
	}
	m.focus = FocusModal
}

func (m *Model) openEdit(item listItem) {
	obs, ok := item.Value.(model.Observation)
	if !ok {
		return
	}
	titleInput := textinput.New()
	titleInput.Prompt = "Title: "
	titleInput.SetValue(obs.Title)
	titleInput.Focus()
	typeInput := textinput.New()
	typeInput.Prompt = "Type: "
	typeInput.SetValue(obs.Type)
	scopeInput := textinput.New()
	scopeInput.Prompt = "Scope: "
	scopeInput.SetValue(obs.Scope)
	topicInput := textinput.New()
	topicInput.Prompt = "Topic: "
	topicInput.SetValue(topicValue(obs.TopicKey))

	content := textarea.New()
	content.Prompt = ""
	content.SetValue(obs.Content)
	content.ShowLineNumbers = false
	content.SetWidth(max(20, m.detail.Width-4))
	content.SetHeight(max(8, m.height/3))
	content.Focus()

	m.edit = &editState{
		Original: obs,
		Fields:   []textinput.Model{titleInput, typeInput, scopeInput, topicInput},
		Content:  content,
		Active:   0,
	}
	m.focus = FocusEdit
}

func (m *Model) refreshDetail() {
	width := max(24, m.width-58)
	height := max(10, m.height-8)
	m.detail.Width = width
	m.detail.Height = height
	m.detail.SetContent(m.detailContent())
}

func (m *Model) detailContent() string {
	if m.loading {
		return "Loading data from Engram…"
	}
	if m.edit != nil {
		return m.renderEditDetail()
	}
	item := m.selectedItem()
	switch {
	case item == nil && m.activeTab == TabDashboard:
		return m.renderDashboardDetail()
	case item == nil:
		return "No results for the current filters."
	}

	switch value := item.Value.(type) {
	case model.DerivedSession:
		return m.renderSessionDetail(value)
	case model.Observation:
		return m.renderObservationDetail(value)
	case model.TopicGroup:
		return m.renderTopicDetail(value)
	case model.Prompt:
		return m.renderPromptDetail(value)
	case model.SessionSummary:
		return m.renderEmptySessionDetail(value)
	default:
		return m.renderDashboardDetail()
	}
}

func (m *Model) renderDashboardDetail() string {
	return strings.Join([]string{
		"Engram Monitor TUI",
		"",
		fmt.Sprintf("Service: %s", m.data.Health.Service),
		fmt.Sprintf("Status:  %s", m.data.Health.Status),
		fmt.Sprintf("Version: %s", m.data.Health.Version),
		"",
		fmt.Sprintf("Sessions:     %d", m.data.Stats.TotalSessions),
		fmt.Sprintf("Observations: %d", m.data.Stats.TotalObservations),
		fmt.Sprintf("Prompts:      %d", m.data.Stats.TotalPrompts),
		fmt.Sprintf("Projects:     %d", len(m.data.Projects)),
		"",
		"Shortcuts",
		"  1-7 switch tab",
		"  tab cycle focus",
		"  / search current view",
		"  p cycle project filter",
		"  t cycle type filter",
		"  o cycle scope filter",
		"  enter drill into sessions/topics",
		"  e edit observation",
		"  d delete prompt or empty session",
		"  x export · i import · m merge",
		"  r reload · q quit",
	}, "\n")
}

func (m *Model) renderSessionDetail(session model.DerivedSession) string {
	lines := []string{
		session.SessionID,
		"",
		fmt.Sprintf("Agent: %s", session.AgentName),
		fmt.Sprintf("Project: %s", session.Project),
		fmt.Sprintf("Last activity: %s", model.PrettyTime(session.Date)),
		fmt.Sprintf("Observations: %d", session.ObservationCount),
		fmt.Sprintf("Types: %s", strings.Join(session.Types, ", ")),
	}
	if session.TopicKey != "" {
		lines = append(lines, fmt.Sprintf("Topic: %s", session.TopicKey))
	}
	if session.SessionSummaryRef != nil && session.SessionSummaryRef.Summary != nil {
		lines = append(lines, "", "Summary", *session.SessionSummaryRef.Summary)
	}
	lines = append(lines, "", "Press enter to open this session.")
	return strings.Join(lines, "\n")
}

func (m *Model) renderObservationDetail(obs model.Observation) string {
	lines := []string{
		fmt.Sprintf("%s  #%d", obs.Type, obs.ID),
		obs.Title,
		"",
		fmt.Sprintf("Project: %s", orFallback(projectOf(obs), "(no project)")),
		fmt.Sprintf("Session: %s", obs.SessionID),
		fmt.Sprintf("Scope:   %s", obs.Scope),
		fmt.Sprintf("Created: %s", model.PrettyTime(obs.CreatedAt)),
		fmt.Sprintf("Updated: %s", model.PrettyTime(obs.UpdatedAt)),
	}
	if obs.TopicKey != nil {
		lines = append(lines, fmt.Sprintf("Topic:   %s", *obs.TopicKey))
	}
	lines = append(lines, "", "Content", obs.Content)
	if m.timeline != nil && m.timeline.Focus.ID == obs.ID {
		lines = append(lines, "", "Timeline")
		for _, entry := range m.timeline.Before {
			lines = append(lines, fmt.Sprintf("  <- %s · %s", entry.Type, entry.Title))
		}
		lines = append(lines, fmt.Sprintf("  ** %s · %s", m.timeline.Focus.Type, m.timeline.Focus.Title))
		for _, entry := range m.timeline.After {
			lines = append(lines, fmt.Sprintf("  -> %s · %s", entry.Type, entry.Title))
		}
	}
	lines = append(lines, "", "Press e to edit or T to load session timeline.")
	return strings.Join(lines, "\n")
}

func (m *Model) renderTopicDetail(topic model.TopicGroup) string {
	lines := []string{
		topic.Key,
		"",
		fmt.Sprintf("Projects: %s", strings.Join(topic.Projects, ", ")),
		fmt.Sprintf("Types: %s", strings.Join(topic.Types, ", ")),
		fmt.Sprintf("Observations: %d", len(topic.Observations)),
		fmt.Sprintf("Last update: %s", model.PrettyTime(topic.LatestDate)),
		"",
		"Press enter to browse observations in this topic.",
	}
	for i, obs := range topic.Observations {
		if i >= 6 {
			lines = append(lines, fmt.Sprintf("…and %d more", len(topic.Observations)-i))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s · %s", obs.Type, truncate(obs.Title, 64)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderPromptDetail(prompt model.Prompt) string {
	return strings.Join([]string{
		fmt.Sprintf("Prompt #%d", prompt.ID),
		"",
		fmt.Sprintf("Project: %s", orFallback(prompt.Project, "(no project)")),
		fmt.Sprintf("Session: %s", prompt.SessionID),
		fmt.Sprintf("Created: %s", model.PrettyTime(prompt.CreatedAt)),
		"",
		prompt.Content,
		"",
		"Press d to delete this prompt.",
	}, "\n")
}

func (m *Model) renderEmptySessionDetail(session model.SessionSummary) string {
	lines := []string{
		"Empty session",
		"",
		fmt.Sprintf("ID: %s", session.ID),
		fmt.Sprintf("Project: %s", orFallback(session.Project, "(no project)")),
		fmt.Sprintf("Started: %s", model.PrettyTime(session.StartedAt)),
		fmt.Sprintf("Observations: %d", session.ObservationCount),
	}
	if session.Summary != nil {
		lines = append(lines, "", *session.Summary)
	}
	lines = append(lines, "", "Press d to delete this empty session.")
	return strings.Join(lines, "\n")
}

func (m *Model) renderEditDetail() string {
	if m.edit == nil {
		return ""
	}
	fields := make([]string, 0, len(m.edit.Fields)+6)
	fields = append(fields, fmt.Sprintf("Editing observation #%d", m.edit.Original.ID), "")
	for i, field := range m.edit.Fields {
		prefix := "  "
		if m.edit.Active == i {
			prefix = "▸ "
		}
		fields = append(fields, prefix+field.View())
	}
	prefix := "  "
	if m.edit.Active == len(m.edit.Fields) {
		prefix = "▸ "
	}
	fields = append(fields, "", prefix+"Content", m.edit.Content.View(), "", "enter save · esc cancel · tab next field")
	return strings.Join(fields, "\n")
}

func (m *Model) selectedObservation() *model.Observation {
	item := m.selectedItem()
	if item == nil {
		return nil
	}
	obs, ok := item.Value.(model.Observation)
	if !ok {
		return nil
	}
	return &obs
}

func (m *Model) applyLoaded(data model.Dataset) {
	m.loading = false
	m.data = data
	m.err = ""
	m.lastLoaded = time.Now()
	m.status = fmt.Sprintf("Loaded %d observations across %d sessions.", len(data.Observations), len(data.Sessions))
	m.timeline = nil
	m.ensureCursor()
	m.refreshDetail()
}

func (m *Model) ensureCursor() {
	items := m.listItems()
	if len(items) == 0 {
		m.cursor = 0
		m.scroll = 0
		return
	}
	if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) saveExport(path, project string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		raw, err := m.client.Export(ctx, project)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: "Export saved to " + path}
	}
}

func (m *Model) doImport(path string) tea.Cmd {
	return func() tea.Msg {
		raw, err := os.ReadFile(path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		var data model.ExportData
		if err := jsonUnmarshal(raw, &data); err != nil {
			return actionDoneMsg{err: err}
		}

		obsDup := 0
		promptDup := 0
		obsSet := make(map[int64]struct{}, len(m.data.Observations))
		for _, obs := range m.data.Observations {
			obsSet[obs.ID] = struct{}{}
		}
		promptSet := make(map[int64]struct{}, len(m.data.Prompts))
		for _, prompt := range m.data.Prompts {
			promptSet[prompt.ID] = struct{}{}
		}
		for _, obs := range data.Observations {
			if _, ok := obsSet[obs.ID]; ok {
				obsDup++
			}
		}
		for _, prompt := range data.Prompts {
			if _, ok := promptSet[prompt.ID]; ok {
				promptDup++
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := m.client.Import(ctx, raw); err != nil {
			return actionDoneMsg{err: err}
		}
		status := fmt.Sprintf("Imported %d sessions, %d observations, %d prompts (possible duplicates by ID: %d obs, %d prompts).",
			len(data.Sessions), len(data.Observations), len(data.Prompts), obsDup, promptDup)
		return actionDoneMsg{status: status, reload: true}
	}
}

func (m *Model) doMerge(from, to string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := m.client.MergeProjects(ctx, from, to); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: fmt.Sprintf("Merged project %q into %q.", from, to), reload: true}
	}
}

func (m *Model) doDelete(item listItem) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		switch value := item.Value.(type) {
		case model.Prompt:
			if err := m.client.DeletePrompt(ctx, value.ID); err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{status: fmt.Sprintf("Deleted prompt #%d.", value.ID), reload: true}
		case model.SessionSummary:
			if err := m.client.DeleteSession(ctx, value.ID); err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{status: fmt.Sprintf("Deleted empty session %q.", value.ID), reload: true}
		default:
			return actionDoneMsg{err: fmt.Errorf("unsupported delete target")}
		}
	}
}

func (m *Model) doSaveEdit() tea.Cmd {
	if m.edit == nil {
		return nil
	}
	original := m.edit.Original
	title := strings.TrimSpace(m.edit.Fields[0].Value())
	typ := strings.TrimSpace(m.edit.Fields[1].Value())
	scope := strings.TrimSpace(m.edit.Fields[2].Value())
	topic := strings.TrimSpace(m.edit.Fields[3].Value())
	content := strings.TrimSpace(m.edit.Content.Value())

	payload := model.ObservationUpdate{}
	changed := false
	if title != original.Title {
		payload.Title = ptr(title)
		changed = true
	}
	if typ != original.Type {
		payload.Type = ptr(typ)
		changed = true
	}
	if scope != original.Scope {
		payload.Scope = ptr(scope)
		changed = true
	}
	if content != original.Content {
		payload.Content = ptr(content)
		changed = true
	}
	var originalTopic string
	if original.TopicKey != nil {
		originalTopic = *original.TopicKey
	}
	if topic != originalTopic {
		payload.TopicKey = ptr(topic)
		changed = true
	}
	if !changed {
		m.edit = nil
		m.focus = FocusList
		m.status = "No observation changes to save."
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		updated, err := m.client.UpdateObservation(ctx, original.ID, payload)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{status: fmt.Sprintf("Updated observation #%d (%s).", updated.ID, updated.Title), reload: true}
	}
}

func ptr[T any](value T) *T { return &value }

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func topicValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func truncate(value string, limit int) string {
	runes := []rune(strings.ReplaceAll(value, "\n", " "))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}

func orFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func projectOf(obs model.Observation) string {
	if obs.Project == nil {
		return ""
	}
	return *obs.Project
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func jsonUnmarshal(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}
