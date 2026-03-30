package tui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YoeDistro/yoe-ng/internal/build"
	"github.com/YoeDistro/yoe-ng/internal/resolve"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
)

// Styles
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cachedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue
	failedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	buildingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	waitingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
)

// Package-level program reference for sending messages from goroutines.
var tuiProgram *tea.Program

// Views
type viewKind int

const (
	viewUnits viewKind = iota
	viewDetail
)

// Unit status
type unitStatus int

const (
	statusNone unitStatus = iota
	statusCached
	statusWaiting  // queued, deps building first
	statusBuilding // actively compiling
	statusFailed
)

// Messages
type tickMsg time.Time

type buildDoneMsg struct {
	unit string
	err  error
}

type buildEventMsg struct {
	unit   string
	status string // "cached", "building", "done", "failed"
}

type execDoneMsg struct {
	err error
}

// model is the Bubble Tea model for the yoe TUI.
type model struct {
	proj       *yoestar.Project
	projectDir string
	units      []string
	hashes     map[string]string
	statuses   map[string]unitStatus
	cursor     int
	view       viewKind
	detailUnit   string
	outputLines  []string // executor output (.output.log)
	logLines     []string // build log (build.log)
	detailScroll int      // scroll offset from top in detail view
	autoFollow   bool     // auto-scroll to bottom during builds
	listOffset   int      // first visible row in unit list
	tick       bool // toggles for flashing indicator
	width      int
	height     int
	message    string
	building   map[string]bool
	cancels    map[string]context.CancelFunc // cancel funcs for active builds
	confirm    string // non-empty = waiting for y/n confirmation
	searching  bool   // true = search input active
	searchText string // current search query
	filtered   []int  // indices into units matching search
}

// Run launches the TUI.
func Run(proj *yoestar.Project, projectDir string) error {
	dag, err := resolve.BuildDAG(proj)
	if err != nil {
		return fmt.Errorf("building DAG: %w", err)
	}

	arch := build.Arch()
	hashes, err := resolve.ComputeAllHashes(dag, arch)
	if err != nil {
		return fmt.Errorf("computing hashes: %w", err)
	}

	units := sortedKeys(proj.Units)
	statuses := make(map[string]unitStatus, len(units))
	for _, name := range units {
		hash := hashes[name]
		if build.IsBuildCached(projectDir, name, hash) {
			statuses[name] = statusCached
		} else if build.HasBuildLog(projectDir, name) {
			statuses[name] = statusFailed
		}
	}

	m := model{
		proj:       proj,
		projectDir: projectDir,
		units:      units,
		hashes:     hashes,
		statuses:   statuses,
		building:   make(map[string]bool),
		cancels:    make(map[string]context.CancelFunc),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	tuiProgram = p
	_, err = p.Run()
	return err
}

func doTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return doTick()
}

// ----- Update -----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.tick = !m.tick
		if m.view == viewDetail {
			m.refreshDetail()
		}
		return m, doTick()

	case buildEventMsg:
		switch msg.status {
		case "cached", "done":
			m.statuses[msg.unit] = statusCached
		case "waiting":
			m.statuses[msg.unit] = statusWaiting
		case "building":
			m.statuses[msg.unit] = statusBuilding
		case "failed":
			m.statuses[msg.unit] = statusFailed
		}
		return m, nil

	case buildDoneMsg:
		delete(m.building, msg.unit)
		delete(m.cancels, msg.unit)
		if msg.err != nil {
			if msg.err.Error() == "build cancelled" || strings.Contains(msg.err.Error(), "signal: killed") {
				m.statuses[msg.unit] = statusNone
				m.message = fmt.Sprintf("Build cancelled: %s", msg.unit)
			} else {
				m.statuses[msg.unit] = statusFailed
				m.message = fmt.Sprintf("Build failed: %s", msg.unit)
			}
		} else {
			m.statuses[msg.unit] = statusCached
			m.message = fmt.Sprintf("Build complete: %s", msg.unit)
		}
		return m, nil

	case execDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Command error: %v", msg.err)
		}
		return m, nil

	case tea.KeyMsg:
		// Handle confirmation prompt
		if m.confirm != "" {
			return m.updateConfirm(msg)
		}
		// Handle search input
		if m.searching {
			return m.updateSearch(msg)
		}
		m.message = ""
		switch m.view {
		case viewUnits:
			return m.updateUnits(msg)
		case viewDetail:
			return m.updateDetail(msg)
		}
	}
	return m, nil
}

func (m model) updateUnits(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Cancel all running builds before quitting
		for name, cancel := range m.cancels {
			cancel()
			delete(m.cancels, name)
		}
		return m, tea.Quit

	case "up", "k":
		m.cursor = m.prevVisible()
		m.adjustListOffset()
		return m, nil

	case "down", "j":
		m.cursor = m.nextVisible()
		m.adjustListOffset()
		return m, nil

	case "pgup":
		vis := m.visibleIndices()
		page := m.listViewportHeight()
		cursorPos := 0
		for vi, i := range vis {
			if i == m.cursor {
				cursorPos = vi
				break
			}
		}
		newPos := cursorPos - page
		if newPos < 0 {
			newPos = 0
		}
		if len(vis) > 0 {
			m.cursor = vis[newPos]
		}
		m.adjustListOffset()
		return m, nil

	case "pgdown":
		vis := m.visibleIndices()
		page := m.listViewportHeight()
		cursorPos := 0
		for vi, i := range vis {
			if i == m.cursor {
				cursorPos = vi
				break
			}
		}
		newPos := cursorPos + page
		if newPos >= len(vis) {
			newPos = len(vis) - 1
		}
		if len(vis) > 0 {
			m.cursor = vis[newPos]
		}
		m.adjustListOffset()
		return m, nil

	case "enter":
		if m.cursor < len(m.units) {
			m.detailUnit = m.units[m.cursor]
			m.view = viewDetail
			m.autoFollow = true
			m.detailScroll = 0
			m.refreshDetail()
			if m.autoFollow {
				m.scrollToBottom()
			}
		}
		return m, nil

	case "b":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			return m, m.startBuild(name)
		}
		return m, nil

	case "x":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			if cancel, ok := m.cancels[name]; ok {
				cancel()
				delete(m.cancels, name)
				m.message = fmt.Sprintf("Cancelling build: %s", name)
			}
		}
		return m, nil

	case "B":
		var cmds []tea.Cmd
		for _, name := range m.units {
			if m.statuses[name] != statusBuilding {
				if cmd := m.startBuild(name); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		return m, tea.Batch(cmds...)

	case "e":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			path := findUnitFile(m.projectDir, name)
			if path != "" {
				return m, m.execEditor(path)
			}
			m.message = fmt.Sprintf("Could not find .star file for %s", name)
		}
		return m, nil

	case "l":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			logPath := filepath.Join(m.projectDir, "build", name, "build.log")
			if _, err := os.Stat(logPath); err == nil {
				return m, m.execEditor(logPath)
			}
			m.message = fmt.Sprintf("No build log for %s", name)
		}
		return m, nil

	case "d":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			logPath := filepath.Join(m.projectDir, "build", name, "build.log")
			c := exec.Command("claude", fmt.Sprintf("diagnose %s", logPath))
			c.Dir = m.projectDir
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err: err}
			})
		}
		return m, nil

	case "a":
		c := exec.Command("claude", "/new-unit")
		c.Dir = m.projectDir
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})

	case "r":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			if u, ok := m.proj.Units[name]; ok && u.Class == "image" {
				c := exec.Command(os.Args[0], "run", name)
				c.Dir = m.projectDir
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return execDoneMsg{err: err}
				})
			}
			m.message = fmt.Sprintf("%s is not an image unit", name)
		}
		return m, nil

	case "c":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			m.confirm = "clean:" + name
			m.message = fmt.Sprintf("Clean %s? All build artifacts will be removed. (y/n)", name)
		}
		return m, nil

	case "C":
		m.confirm = "clean-all"
		m.message = "Clean ALL build artifacts? (y/n)"
		return m, nil

	case "/":
		m.searching = true
		m.searchText = ""
		m.filtered = nil
		return m, nil
	}
	return m, nil
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchText = ""
		m.filtered = nil
		return m, nil

	case "enter":
		m.searching = false
		// Keep filter active, cursor stays on first match
		if len(m.filtered) > 0 {
			m.cursor = m.filtered[0]
		}
		return m, nil

	case "backspace":
		if len(m.searchText) > 0 {
			m.searchText = m.searchText[:len(m.searchText)-1]
			m.applyFilter()
		}
		return m, nil

	default:
		// Single printable character
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.searchText += key
			m.applyFilter()
		}
		return m, nil
	}
}

func (m *model) applyFilter() {
	m.filtered = nil
	if m.searchText == "" {
		return
	}
	query := strings.ToLower(m.searchText)
	for i, name := range m.units {
		if strings.Contains(strings.ToLower(name), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	// Move cursor to first match
	if len(m.filtered) > 0 {
		m.cursor = m.filtered[0]
	}
	m.listOffset = 0
	m.adjustListOffset()
}

func (m model) visibleIndices() []int {
	if m.searchText != "" && m.filtered != nil {
		return m.filtered
	}
	idx := make([]int, len(m.units))
	for i := range idx {
		idx[i] = i
	}
	return idx
}

func (m model) prevVisible() int {
	vis := m.visibleIndices()
	for i := len(vis) - 1; i >= 0; i-- {
		if vis[i] < m.cursor {
			return vis[i]
		}
	}
	return m.cursor
}

func (m model) nextVisible() int {
	vis := m.visibleIndices()
	for _, idx := range vis {
		if idx > m.cursor {
			return idx
		}
	}
	return m.cursor
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirm
	m.confirm = ""

	switch msg.String() {
	case "y", "Y":
		if strings.HasPrefix(action, "clean:") {
			name := strings.TrimPrefix(action, "clean:")
			buildDir := filepath.Join(m.projectDir, "build", name)
			if err := os.RemoveAll(buildDir); err != nil {
				m.message = fmt.Sprintf("Clean failed: %v", err)
			} else {
				m.statuses[name] = statusNone
				m.message = fmt.Sprintf("Cleaned %s", name)
			}
		} else if action == "clean-all" {
			buildDir := filepath.Join(m.projectDir, "build")
			if err := os.RemoveAll(buildDir); err != nil {
				m.message = fmt.Sprintf("Clean failed: %v", err)
			} else {
				for _, name := range m.units {
					m.statuses[name] = statusNone
				}
				m.message = "Cleaned all build artifacts"
			}
		}
	default:
		m.message = "Cancelled"
	}
	return m, nil
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewUnits
		m.detailUnit = ""
		m.outputLines = nil
		m.logLines = nil
		m.detailScroll = 0
		return m, nil

	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		m.autoFollow = false
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return m, nil

	case "down", "j":
		maxScroll := m.detailMaxScroll()
		if m.detailScroll < maxScroll {
			m.detailScroll++
		}
		if m.detailScroll >= maxScroll {
			m.autoFollow = true
		}
		return m, nil

	case "pgup":
		m.autoFollow = false
		page := m.detailViewportHeight()
		m.detailScroll -= page
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
		return m, nil

	case "pgdown":
		page := m.detailViewportHeight()
		maxScroll := m.detailMaxScroll()
		m.detailScroll += page
		if m.detailScroll > maxScroll {
			m.detailScroll = maxScroll
		}
		if m.detailScroll >= maxScroll {
			m.autoFollow = true
		}
		return m, nil

	case "G":
		m.autoFollow = true
		m.scrollToBottom()
		return m, nil

	case "g":
		m.autoFollow = false
		m.detailScroll = 0
		return m, nil

	case "b":
		m.autoFollow = true
		return m, m.startBuild(m.detailUnit)

	case "d":
		logPath := filepath.Join(m.projectDir, "build", m.detailUnit, "build.log")
		c := exec.Command("claude", fmt.Sprintf("diagnose %s", logPath))
		c.Dir = m.projectDir
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})

	case "r":
		if u, ok := m.proj.Units[m.detailUnit]; ok && u.Class == "image" {
			c := exec.Command(os.Args[0], "run", m.detailUnit)
			c.Dir = m.projectDir
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err: err}
			})
		}
		m.message = fmt.Sprintf("%s is not an image unit", m.detailUnit)
		return m, nil

	case "l":
		logPath := filepath.Join(m.projectDir, "build", m.detailUnit, "build.log")
		if _, err := os.Stat(logPath); err == nil {
			return m, m.execEditor(logPath)
		}
		m.message = fmt.Sprintf("No build log for %s", m.detailUnit)
		return m, nil
	}
	return m, nil
}

// ----- View -----

func (m model) View() string {
	switch m.view {
	case viewDetail:
		return m.viewDetail()
	default:
		return m.viewUnits()
	}
}

func (m model) viewUnits() string {
	var b strings.Builder

	// Header
	machine := m.proj.Defaults.Machine
	image := m.proj.Defaults.Image
	b.WriteString(fmt.Sprintf("  %s  Machine: %s  Image: %s\n\n",
		titleStyle.Render("Yoe-NG"),
		headerStyle.Render(machine),
		headerStyle.Render(image)))

	// Column header
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		headerStyle.Render(fmt.Sprintf("%-28s", "NAME")),
		headerStyle.Render(fmt.Sprintf("%-12s", "CLASS")),
		headerStyle.Render("STATUS")))

	// Determine visible units — filtered if search active, all otherwise
	visible := make([]int, 0, len(m.units))
	if m.searchText != "" && m.filtered != nil {
		visible = m.filtered
	} else {
		for i := range m.units {
			visible = append(visible, i)
		}
	}

	// Calculate visible window for unit list
	maxRows := m.listViewportHeight()
	end := m.listOffset + maxRows
	if end > len(visible) {
		end = len(visible)
	}

	if m.listOffset > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more", m.listOffset)))
		b.WriteString("\n")
	}

	for _, i := range visible[m.listOffset:end] {
		name := m.units[i]
		cursor := "  "
		nameStyle := dimStyle
		if i == m.cursor {
			cursor = "→ "
			nameStyle = selectedStyle
		}

		class := ""
		if u, ok := m.proj.Units[name]; ok {
			class = u.Class
		}

		status := m.renderStatus(name)

		paddedName := fmt.Sprintf("%-28s", name)
		paddedClass := fmt.Sprintf("%-12s", class)
		b.WriteString(fmt.Sprintf("%s%s %s %s\n",
			cursor,
			nameStyle.Render(paddedName),
			dimStyle.Render(paddedClass),
			status))
	}

	if end < len(visible) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more", len(visible)-end)))
		b.WriteString("\n")
	}

	// Search bar or help bar
	b.WriteString("\n")
	if m.searching {
		b.WriteString(fmt.Sprintf("  /%s▌", m.searchText))
	} else {
		help := "  b build  x cancel  e edit  d diagnose  l log  c clean  / search  q quit"
		if m.cursor < len(m.units) {
			if u, ok := m.proj.Units[m.units[m.cursor]]; ok && u.Class == "image" {
				help = "  b build  x cancel  r run  e edit  d diagnose  l log  c clean  / search  q quit"
			}
		}
		b.WriteString(helpStyle.Render(help))
	}
	b.WriteString("\n")

	// Status message
	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("  "+m.message))
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) viewDetail() string {
	var b strings.Builder

	status := m.renderStatus(m.detailUnit)
	b.WriteString(fmt.Sprintf("  ← %s %s\n\n",
		titleStyle.Render(m.detailUnit),
		status))

	// Build combined content lines for scrolling
	var allLines []string

	// Build output section
	allLines = append(allLines, headerStyle.Render("  BUILD OUTPUT"))
	if len(m.outputLines) == 0 {
		allLines = append(allLines, dimStyle.Render("  (no output yet)"))
	} else {
		for _, line := range m.outputLines {
			allLines = append(allLines, "  "+line)
		}
	}

	// Separator
	allLines = append(allLines, "")

	// Build log section
	allLines = append(allLines, headerStyle.Render("  BUILD LOG"))
	if len(m.logLines) == 0 {
		allLines = append(allLines, dimStyle.Render("  (no build log)"))
	} else {
		for _, line := range m.logLines {
			allLines = append(allLines, "  "+line)
		}
	}

	// Calculate visible window
	viewH := m.detailViewportHeight()
	start := m.detailScroll
	if start > len(allLines)-viewH {
		start = len(allLines) - viewH
	}
	if start < 0 {
		start = 0
	}
	end := start + viewH
	if end > len(allLines) {
		end = len(allLines)
	}

	for _, line := range allLines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad remaining lines so help bar stays at bottom
	rendered := end - start
	for i := rendered; i < viewH; i++ {
		b.WriteString("\n")
	}

	// Scroll indicator
	scrollInfo := ""
	if len(allLines) > viewH {
		pct := 100
		if len(allLines)-viewH > 0 {
			pct = start * 100 / (len(allLines) - viewH)
		}
		if m.autoFollow {
			scrollInfo = dimStyle.Render(fmt.Sprintf("  [auto-follow] %d%%", pct))
		} else {
			scrollInfo = dimStyle.Render(fmt.Sprintf("  [%d/%d] %d%%", start+1, len(allLines), pct))
		}
	}
	b.WriteString(scrollInfo)
	b.WriteString("\n")

	// Help bar
	help := "  esc back  j/k scroll  g top  G bottom  b build  d diagnose  l log"
	if u, ok := m.proj.Units[m.detailUnit]; ok && u.Class == "image" {
		help = "  esc back  j/k scroll  g top  G bottom  b build  r run  d diagnose  l log"
	}
	b.WriteString(helpStyle.Render(help))
	b.WriteString("\n")

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("  "+m.message))
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderStatus(name string) string {
	switch m.statuses[name] {
	case statusCached:
		return cachedStyle.Render("● cached")
	case statusWaiting:
		return waitingStyle.Render("● waiting")
	case statusBuilding:
		if m.tick {
			return buildingStyle.Render("▌building...")
		}
		return "            " // blank when flashing off
	case statusFailed:
		return failedStyle.Render("● failed")
	default:
		return ""
	}
}

// ----- Helpers -----

func (m *model) startBuild(name string) tea.Cmd {
	if m.statuses[name] == statusBuilding || m.statuses[name] == statusWaiting {
		return nil
	}
	m.statuses[name] = statusWaiting
	m.building[name] = true

	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[name] = cancel

	proj := m.proj
	projectDir := m.projectDir
	unitName := name

	// Write executor output to a log file so detail view can tail it
	outputPath := filepath.Join(projectDir, "build", unitName, ".output.log")
	build.EnsureDir(filepath.Dir(outputPath))

	return func() tea.Msg {
		defer cancel()
		f, err := os.Create(outputPath)
		if err != nil {
			return buildDoneMsg{unit: unitName, err: err}
		}
		defer f.Close()

		err = build.BuildUnits(proj, []string{unitName}, build.Options{
			Ctx:        ctx,
			Force:      true,
			ProjectDir: projectDir,
			Arch:       build.Arch(),
			OnEvent: func(ev build.BuildEvent) {
				if tuiProgram != nil {
					tuiProgram.Send(buildEventMsg{
						unit:   ev.Unit,
						status: ev.Status,
					})
				}
			},
		}, f)
		return buildDoneMsg{unit: unitName, err: err}
	}
}

func (m model) execEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{err: err}
	})
}

func (m *model) refreshDetail() {
	outputPath := filepath.Join(m.projectDir, "build", m.detailUnit, ".output.log")
	m.outputLines = readFileAll(outputPath)
	logPath := filepath.Join(m.projectDir, "build", m.detailUnit, "build.log")
	m.logLines = readFileAll(logPath)
	if m.autoFollow {
		m.scrollToBottom()
	}
}

// detailViewportHeight returns the number of content lines visible in detail view.
// Reserves lines for: header (2) + scroll indicator (1) + help bar (1) + message (2) + padding (1).
func (m model) detailViewportHeight() int {
	h := m.height - 7
	if h < 5 {
		h = 5
	}
	return h
}

// detailTotalLines returns the total number of lines in the combined detail content.
func (m model) detailTotalLines() int {
	n := 1 // BUILD OUTPUT header
	if len(m.outputLines) == 0 {
		n++ // "(no output yet)"
	} else {
		n += len(m.outputLines)
	}
	n++ // separator
	n++ // BUILD LOG header
	if len(m.logLines) == 0 {
		n++ // "(no build log)"
	} else {
		n += len(m.logLines)
	}
	return n
}

// detailMaxScroll returns the maximum scroll offset for the detail view.
func (m model) detailMaxScroll() int {
	max := m.detailTotalLines() - m.detailViewportHeight()
	if max < 0 {
		return 0
	}
	return max
}

// scrollToBottom sets the scroll position to the end of content.
func (m *model) scrollToBottom() {
	m.detailScroll = m.detailMaxScroll()
}

// adjustListOffset ensures the cursor is visible within the unit list viewport.
func (m *model) adjustListOffset() {
	visible := m.visibleIndices()
	maxRows := m.listViewportHeight()

	cursorPos := -1
	for vi, i := range visible {
		if i == m.cursor {
			cursorPos = vi
			break
		}
	}
	if cursorPos < 0 {
		return
	}
	if cursorPos < m.listOffset {
		m.listOffset = cursorPos
	}
	if cursorPos >= m.listOffset+maxRows {
		m.listOffset = cursorPos - maxRows + 1
	}
	if m.listOffset > len(visible)-maxRows {
		m.listOffset = len(visible) - maxRows
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

// listViewportHeight returns the number of unit rows that fit on screen.
func (m model) listViewportHeight() int {
	h := m.height - 8
	if h < 5 {
		h = 5
	}
	return h
}

func findUnitFile(projectDir, name string) string {
	// Search roots: project's own layers + parent directories for relative
	// layer references (e.g., testdata/e2e-project references ../../layers/)
	roots := []string{projectDir}
	for _, rel := range []string{"..", filepath.Join("..", "..")} {
		r := filepath.Join(projectDir, rel)
		if _, err := os.Stat(filepath.Join(r, "layers")); err == nil {
			roots = append(roots, r)
		}
	}

	// First pass: look for an exact <name>.star file
	for _, root := range roots {
		layersDir := filepath.Join(root, "layers")
		var found string
		filepath.WalkDir(layersDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if d.Name() == name+".star" {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}

	// Second pass: derived units (e.g., base-files-dev is defined inside
	// another .star file via a function call). Grep for the name string.
	needle := []byte(`"` + name + `"`)
	for _, root := range roots {
		layersDir := filepath.Join(root, "layers")
		var found string
		filepath.WalkDir(layersDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".star") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if bytes.Contains(data, needle) {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}

	return ""
}

func readFileAll(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // handle long lines up to 1MB
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
