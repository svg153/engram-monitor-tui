package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary = lipgloss.Color("#89B4FA")
	colorAccent  = lipgloss.Color("#CBA6F7")
	colorOk      = lipgloss.Color("#A6E3A1")
	colorWarn    = lipgloss.Color("#F9E2AF")
	colorError   = lipgloss.Color("#F38BA8")
	colorMuted   = lipgloss.Color("#6C7086")
	colorPanel   = lipgloss.Color("#1E1E2E")
	colorBorder  = lipgloss.Color("#45475A")

	appStyle = lipgloss.NewStyle().Padding(1, 1)
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)
	activeBoxStyle = boxStyle.Copy().BorderForeground(colorAccent)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	mutedStyle     = lipgloss.NewStyle().Foreground(colorMuted)
	statusStyle    = lipgloss.NewStyle().Foreground(colorOk)
	errorStyle     = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	selectedStyle  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	tabStyle       = lipgloss.NewStyle().Padding(0, 1)
	activeTabStyle = tabStyle.Copy().Background(colorAccent).Foreground(lipgloss.Color("#11111B")).Bold(true)
	modalStyle     = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			Width(72)
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Starting engram-monitor-tui…"
	}

	sidebarWidth := 20
	listWidth := max(36, m.width/3)
	detailWidth := max(34, m.width-sidebarWidth-listWidth-8)

	sidebar := activeBoxStyle.Width(sidebarWidth).Height(max(10, m.height-6)).Render(m.renderSidebar())
	list := activeBoxStyle.Width(listWidth).Height(max(10, m.height-6)).Render(m.renderList())
	m.refreshDetail()
	detail := activeBoxStyle.Width(detailWidth).Height(max(10, m.height-6)).Render(m.detail.View())

	header := m.renderHeader()
	footer := m.renderFooter()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list, detail)
	view := appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
	if m.modal != nil {
		return view + "\n" + lipgloss.PlaceHorizontal(m.width-4, lipgloss.Center, m.renderModal())
	}
	return view
}

func (m Model) renderHeader() string {
	statusColor := statusStyle
	if m.loading {
		statusColor = mutedStyle
	}
	if m.err != "" {
		statusColor = errorStyle
	}

	headerLeft := titleStyle.Render("engram-monitor-tui") + " " + mutedStyle.Render("Bubble Tea admin for Engram")
	var stats string
	if m.loading {
		stats = m.spin.View() + " loading"
	} else {
		stats = fmt.Sprintf("%d sessions · %d observations · %d prompts",
			m.data.Stats.TotalSessions,
			m.data.Stats.TotalObservations,
			m.data.Stats.TotalPrompts,
		)
	}
	line1 := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/2).Render(headerLeft),
		lipgloss.NewStyle().Width(m.width/2-4).Align(lipgloss.Right).Render(statusColor.Render(stats)),
	)

	filters := fmt.Sprintf("project=%s · type=%s · scope=%s · query=%q",
		orFallback(m.projectFilterValue(), "all"),
		orFallback(m.typeFilterValue(), "all"),
		orFallback(m.scopeFilterValue(), "all"),
		m.search.Value(),
	)
	statusText := m.status
	if m.err != "" {
		statusText = m.err
	}

	return boxStyle.Width(m.width - 4).Render(strings.Join([]string{
		line1,
		mutedStyle.Render(filters),
		statusColor.Render(statusText),
	}, "\n"))
}

func (m Model) renderSidebar() string {
	lines := []string{titleStyle.Render("Views"), ""}
	for i, tab := range orderedTabs {
		label := fmt.Sprintf("%d. %s", i+1, prettyTab(tab))
		if tab == m.activeTab {
			label = activeTabStyle.Render(label)
		} else if m.focus == FocusSidebar && tab == orderedTabs[(indexOfTab(m.activeTab)+len(orderedTabs))%len(orderedTabs)] {
			label = selectedStyle.Render(label)
		}
		lines = append(lines, label)
	}
	lines = append(lines,
		"",
		titleStyle.Render("Context"),
		orFallback(m.currentSession, orFallback(m.currentTopic, "root")),
		"",
		titleStyle.Render("Focus"),
		m.focusLabel(),
	)
	return strings.Join(lines, "\n")
}

func (m Model) renderList() string {
	items := m.listItems()
	header := prettyTab(m.activeTab)
	if m.currentSession != "" {
		header += " / session"
	}
	if m.currentTopic != "" {
		header += " / topic"
	}

	lines := []string{titleStyle.Render(header), ""}
	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render("No results."))
		return strings.Join(lines, "\n")
	}

	visible := max(5, m.height-14)
	start := min(m.scroll, max(0, len(items)-1))
	end := min(len(items), start+visible)
	for i := start; i < end; i++ {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = "▸ "
			style = selectedStyle
		}
		lines = append(lines, style.Render(prefix+truncate(items[i].Title, 56)))
		lines = append(lines, mutedStyle.Render("   "+truncate(items[i].Subtitle, 56)))
		lines = append(lines, mutedStyle.Render("   "+truncate(items[i].Meta, 56)))
		lines = append(lines, "")
	}
	lines = append(lines, mutedStyle.Render(fmt.Sprintf("showing %d-%d of %d", start+1, end, len(items))))
	return strings.Join(lines, "\n")
}

func (m Model) renderFooter() string {
	help := "tab focus · j/k move · / search · p/t/o filters · enter open · esc back · e edit · d delete · T timeline · x export · i import · m merge · r reload · q quit"
	if m.focus == FocusSearch {
		help = "type to filter current view · enter apply · esc close"
	}
	if m.focus == FocusEdit {
		help = "tab next field · enter save when content focused · esc cancel"
	}
	if m.focus == FocusModal {
		help = "tab next input · enter confirm · esc close"
	}
	return boxStyle.Width(m.width - 4).Render(mutedStyle.Render(help))
}

func (m Model) renderModal() string {
	if m.modal == nil {
		return ""
	}
	lines := []string{titleStyle.Render(m.modal.Title), "", m.modal.Description}
	for _, input := range m.modal.Inputs {
		lines = append(lines, "", input.View())
	}
	button := m.modal.ConfirmLabel
	if m.modal.Danger {
		button = errorStyle.Render(button)
	} else {
		button = selectedStyle.Render(button)
	}
	lines = append(lines, "", fmt.Sprintf("Press enter to %s, esc to cancel.", button))
	return modalStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) focusLabel() string {
	switch m.focus {
	case FocusSidebar:
		return "sidebar"
	case FocusList:
		return "list"
	case FocusDetail:
		return "detail"
	case FocusSearch:
		return "search"
	case FocusEdit:
		return "edit"
	case FocusModal:
		return "modal"
	default:
		return "unknown"
	}
}

func prettyTab(tab Tab) string {
	switch tab {
	case TabDashboard:
		return "Dashboard"
	case TabSessions:
		return "Sessions"
	case TabMemories:
		return "Memories"
	case TabTopics:
		return "Topics"
	case TabTimeline:
		return "Timeline"
	case TabPrompts:
		return "Prompts"
	case TabEmpty:
		return "Empty Sessions"
	default:
		return string(tab)
	}
}

func indexOfTab(tab Tab) int {
	for i, item := range orderedTabs {
		if item == tab {
			return i
		}
	}
	return 0
}
