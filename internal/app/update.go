package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/svg153/engram-monitor-tui/internal/model"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.refreshDetail()
		return m, nil
	case loadMsg:
		if msg.err != nil {
			m.loading = false
			m.err = msg.err.Error()
			m.status = "Failed to load data."
			m.refreshDetail()
			return m, nil
		}
		m.applyLoaded(msg.data)
		return m, nil
	case actionDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Action failed."
			return m, nil
		}
		m.err = ""
		m.status = msg.status
		m.modal = nil
		if m.edit != nil {
			m.edit = nil
			m.focus = FocusList
		}
		if msg.reload {
			m.loading = true
			return m, m.loadAll()
		}
		m.refreshDetail()
		return m, nil
	case timelineMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.timeline = &msg.result
		m.status = fmt.Sprintf("Loaded timeline for observation #%d.", msg.result.Focus.ID)
		m.refreshDetail()
		return m, nil
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.focus == FocusModal && m.modal != nil {
		return m.updateModal(key)
	}
	if m.focus == FocusEdit && m.edit != nil {
		return m.updateEdit(key)
	}
	if m.focus == FocusSearch {
		return m.updateSearch(key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		m.loading = true
		m.status = "Reloading from Engram…"
		return m, m.loadAll()
	case "tab":
		m.focus = (m.focus + 1) % 3
		if m.focus == FocusSearch || m.focus == FocusEdit || m.focus == FocusModal {
			m.focus = FocusSidebar
		}
		return m, nil
	case "shift+tab":
		if m.focus == FocusSidebar {
			m.focus = FocusDetail
		} else {
			m.focus--
		}
		return m, nil
	case "/":
		m.focus = FocusSearch
		m.search.Focus()
		return m, nil
	case "p":
		if len(m.data.Projects) > 0 {
			m.projectFilter = (m.projectFilter + 1) % (len(m.data.Projects) + 1)
		}
		m.cursor, m.scroll = 0, 0
		m.refreshDetail()
		return m, nil
	case "t":
		if m.focus == FocusList || m.focus == FocusSidebar || m.focus == FocusDetail {
			if len(m.data.Types) > 0 {
				m.typeFilter = (m.typeFilter + 1) % (len(m.data.Types) + 1)
			}
			m.cursor, m.scroll = 0, 0
			m.refreshDetail()
			return m, nil
		}
	case "o":
		m.scopeFilter = (m.scopeFilter + 1) % 4
		m.cursor, m.scroll = 0, 0
		m.refreshDetail()
		return m, nil
	case "esc":
		if m.currentSession != "" {
			m.currentSession = ""
			m.cursor, m.scroll = 0, 0
			m.refreshDetail()
			return m, nil
		}
		if m.currentTopic != "" {
			m.currentTopic = ""
			m.cursor, m.scroll = 0, 0
			m.refreshDetail()
			return m, nil
		}
	case "x":
		m.openExportModal()
		return m, nil
	case "i":
		m.openImportModal()
		return m, nil
	case "m":
		m.openMergeModal()
		return m, nil
	case "e":
		if item := m.selectedItem(); item != nil && item.Kind == "observation" {
			m.openEdit(*item)
		}
		return m, nil
	case "d":
		if item := m.selectedItem(); item != nil && (item.Kind == "prompt" || item.Kind == "empty-session") {
			m.openDeleteConfirm(*item)
		}
		return m, nil
	case "T":
		if obs := m.selectedObservation(); obs != nil {
			return m, m.loadTimeline(obs.ID)
		}
	case "enter":
		if item := m.selectedItem(); item != nil {
			switch value := item.Value.(type) {
			case model.DerivedSession:
				m.currentSession = value.SessionID
				m.cursor, m.scroll = 0, 0
				m.refreshDetail()
				return m, nil
			case model.TopicGroup:
				m.currentTopic = value.Key
				m.cursor, m.scroll = 0, 0
				m.refreshDetail()
				return m, nil
			}
		}
	case "1", "2", "3", "4", "5", "6", "7":
		idx := int(key.String()[0] - '1')
		if idx >= 0 && idx < len(orderedTabs) {
			m.activeTab = orderedTabs[idx]
			m.currentSession = ""
			m.currentTopic = ""
			m.cursor, m.scroll = 0, 0
			m.refreshDetail()
			return m, nil
		}
	}

	if m.focus == FocusSidebar {
		switch key.String() {
		case "j", "down":
			m.switchTab(1)
		case "k", "up":
			m.switchTab(-1)
		}
		m.refreshDetail()
		return m, nil
	}
	if m.focus == FocusDetail {
		switch key.String() {
		case "j", "down":
			m.detail.LineDown(1)
		case "k", "up":
			m.detail.LineUp(1)
		case "pgdown":
			m.detail.ViewDown()
		case "pgup":
			m.detail.ViewUp()
		}
		return m, nil
	}
	return m.updateList(key)
}

func (m Model) updateSearch(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.search.Blur()
		m.focus = FocusList
		return m, nil
	case "enter":
		m.search.Blur()
		m.focus = FocusList
		m.cursor, m.scroll = 0, 0
		m.refreshDetail()
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(key)
	m.cursor, m.scroll = 0, 0
	m.refreshDetail()
	return m, cmd
}

func (m Model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.listItems()
	visible := max(5, m.height-10)
	switch key.String() {
	case "j", "down":
		if m.cursor < len(items)-1 {
			m.cursor++
			if m.cursor >= m.scroll+visible {
				m.scroll = m.cursor - visible + 1
			}
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
		}
	case "home":
		m.cursor, m.scroll = 0, 0
	case "end":
		if len(items) > 0 {
			m.cursor = len(items) - 1
			m.scroll = max(0, len(items)-visible)
		}
	}
	m.refreshDetail()
	return m, nil
}

func (m Model) updateEdit(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.edit == nil {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.edit = nil
		m.focus = FocusList
		m.refreshDetail()
		return m, nil
	case "tab":
		m.edit.Active = (m.edit.Active + 1) % (len(m.edit.Fields) + 1)
		return m.syncEditFocus(), nil
	case "shift+tab":
		m.edit.Active--
		if m.edit.Active < 0 {
			m.edit.Active = len(m.edit.Fields)
		}
		return m.syncEditFocus(), nil
	case "enter":
		if m.edit.Active == len(m.edit.Fields) {
			return m, m.doSaveEdit()
		}
	}
	if m.edit.Active == len(m.edit.Fields) {
		var cmd tea.Cmd
		m.edit.Content, cmd = m.edit.Content.Update(key)
		m.refreshDetail()
		return m, cmd
	}
	var cmd tea.Cmd
	m.edit.Fields[m.edit.Active], cmd = m.edit.Fields[m.edit.Active].Update(key)
	m.refreshDetail()
	return m, cmd
}

func (m Model) syncEditFocus() Model {
	if m.edit == nil {
		return m
	}
	for i := range m.edit.Fields {
		if i == m.edit.Active {
			m.edit.Fields[i].Focus()
		} else {
			m.edit.Fields[i].Blur()
		}
	}
	if m.edit.Active == len(m.edit.Fields) {
		m.edit.Content.Focus()
		m.edit.Content.CursorEnd()
	} else {
		m.edit.Content.Blur()
	}
	m.refreshDetail()
	return m
}

func (m Model) updateModal(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal == nil {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.modal = nil
		m.focus = FocusList
		return m, nil
	case "tab":
		if len(m.modal.Inputs) > 0 {
			m.modal.Active = (m.modal.Active + 1) % len(m.modal.Inputs)
			m.syncModalFocus()
		}
		return m, nil
	case "shift+tab":
		if len(m.modal.Inputs) > 0 {
			m.modal.Active--
			if m.modal.Active < 0 {
				m.modal.Active = len(m.modal.Inputs) - 1
			}
			m.syncModalFocus()
		}
		return m, nil
	case "enter":
		switch m.modal.Kind {
		case modalExport:
			return m, m.saveExport(strings.TrimSpace(m.modal.Inputs[0].Value()), strings.TrimSpace(m.modal.Inputs[1].Value()))
		case modalImport:
			return m, m.doImport(strings.TrimSpace(m.modal.Inputs[0].Value()))
		case modalMerge:
			return m, m.doMerge(strings.TrimSpace(m.modal.Inputs[0].Value()), strings.TrimSpace(m.modal.Inputs[1].Value()))
		case modalConfirm:
			item, _ := m.modal.Payload.(listItem)
			return m, m.doDelete(item)
		}
	}
	if len(m.modal.Inputs) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.modal.Inputs[m.modal.Active], cmd = m.modal.Inputs[m.modal.Active].Update(key)
	return m, cmd
}

func (m *Model) syncModalFocus() {
	if m.modal == nil {
		return
	}
	for i := range m.modal.Inputs {
		if i == m.modal.Active {
			m.modal.Inputs[i].Focus()
		} else {
			m.modal.Inputs[i].Blur()
		}
	}
}

func (m *Model) switchTab(delta int) {
	index := 0
	for i, tab := range orderedTabs {
		if tab == m.activeTab {
			index = i
			break
		}
	}
	index = (index + delta + len(orderedTabs)) % len(orderedTabs)
	m.activeTab = orderedTabs[index]
	m.currentSession = ""
	m.currentTopic = ""
	m.cursor, m.scroll = 0, 0
}

func (m Model) loadTimeline(observationID int64) tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.Timeline(context.Background(), observationID, 5, 5)
		return timelineMsg{result: result, err: err}
	}
}
