package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

type view int

const (
	viewMain view = iota
	viewMachines
	viewUnits
)

type model struct {
	proj       *yoestar.Project
	projectDir string
	cursor     int
	view       view
	machines   []string
	units      []string
	selMachine string
	selImage   string
	width      int
	height     int
	message    string
}

type menuItem struct {
	key   string
	label string
}

func mainMenu() []menuItem {
	return []menuItem{
		{"b", "Build all units"},
		{"i", "Build image"},
		{"m", "Select machine"},
		{"r", "Browse units"},
		{"c", "Config"},
		{"q", "Quit"},
	}
}

// Run launches the TUI.
func Run(proj *yoestar.Project, projectDir string) error {
	machines := sortedKeys(proj.Machines)
	units := sortedKeys(proj.Units)

	selMachine := proj.Defaults.Machine
	selImage := proj.Defaults.Image

	m := model{
		proj:       proj,
		projectDir: projectDir,
		machines:   machines,
		units:      units,
		selMachine: selMachine,
		selImage:   selImage,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		m.message = ""

		switch m.view {
		case viewMain:
			return m.updateMain(msg)
		case viewMachines:
			return m.updateList(msg, m.machines, func(sel string) {
				m.selMachine = sel
				m.message = fmt.Sprintf("Machine set to %s", sel)
			})
		case viewUnits:
			return m.updateList(msg, m.units, nil)
		}
	}

	return m, nil
}

func (m model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "b":
		m.message = "Run: yoe build --all"
		return m, tea.Quit
	case "i":
		if m.selImage != "" {
			m.message = fmt.Sprintf("Run: yoe build %s --machine %s", m.selImage, m.selMachine)
		} else {
			m.message = "No default image set"
		}
		return m, tea.Quit
	case "m":
		m.view = viewMachines
		m.cursor = 0
		return m, nil
	case "r":
		m.view = viewUnits
		m.cursor = 0
		return m, nil
	case "c":
		m.message = "Run: yoe config show"
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg, items []string, onSelect func(string)) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.view = viewMain
		m.cursor = 0
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(items)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		if m.cursor < len(items) && onSelect != nil {
			onSelect(items[m.cursor])
		}
		m.view = viewMain
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("  Yoe-NG"))
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(fmt.Sprintf("  Machine: %s    Image: %s\n",
		headerStyle.Render(m.selMachine),
		headerStyle.Render(m.selImage)))
	b.WriteString(fmt.Sprintf("  Machines: %d    Units: %d\n",
		len(m.machines), len(m.units)))
	b.WriteString("\n")

	switch m.view {
	case viewMain:
		for _, item := range mainMenu() {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				selectedStyle.Render("["+item.key+"]"),
				item.label))
		}
	case viewMachines:
		b.WriteString(headerStyle.Render("  Select Machine:"))
		b.WriteString("\n\n")
		for i, name := range m.machines {
			cursor := "  "
			style := dimStyle
			if i == m.cursor {
				cursor = "→ "
				style = selectedStyle
			}
			arch := ""
			if mc, ok := m.proj.Machines[name]; ok {
				arch = mc.Arch
			}
			b.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, style.Render(name), dimStyle.Render(arch)))
		}
		b.WriteString(dimStyle.Render("\n  ↑↓/jk navigate  enter select  esc back"))
	case viewUnits:
		b.WriteString(headerStyle.Render("  Units:"))
		b.WriteString("\n\n")
		for i, name := range m.units {
			cursor := "  "
			style := dimStyle
			if i == m.cursor {
				cursor = "→ "
				style = selectedStyle
			}
			class := ""
			if r, ok := m.proj.Units[name]; ok {
				class = r.Class
			}
			b.WriteString(fmt.Sprintf("  %s%-20s %s\n", cursor, style.Render(name), dimStyle.Render("["+class+"]")))
		}
		b.WriteString(dimStyle.Render("\n  ↑↓/jk navigate  esc back"))
	}

	// Status message
	if m.message != "" {
		b.WriteString("\n\n")
		b.WriteString(statusStyle.Render("  " + m.message))
	}

	b.WriteString("\n")
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
