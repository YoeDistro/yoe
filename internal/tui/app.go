package tui

import (
	"bufio"
	"bytes"
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
	detailUnit  string
	outputLines []string // executor output (.output.log)
	logLines    []string // build log (build.log)
	tick       bool // toggles for flashing indicator
	width      int
	height     int
	message    string
	building   map[string]bool
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
		case "building":
			m.statuses[msg.unit] = statusBuilding
		case "failed":
			m.statuses[msg.unit] = statusFailed
		}
		return m, nil

	case buildDoneMsg:
		delete(m.building, msg.unit)
		if msg.err != nil {
			m.statuses[msg.unit] = statusFailed
			m.message = fmt.Sprintf("Build failed: %s", msg.unit)
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
		// Kill running builds
		return m, tea.Quit

	case "up", "k":
		m.cursor = m.prevVisible()
		return m, nil

	case "down", "j":
		m.cursor = m.nextVisible()
		return m, nil

	case "enter":
		if m.cursor < len(m.units) {
			m.detailUnit = m.units[m.cursor]
			m.view = viewDetail
			m.refreshDetail()
		}
		return m, nil

	case "b":
		if m.cursor < len(m.units) {
			name := m.units[m.cursor]
			return m, m.startBuild(name)
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
		return m, nil

	case "q", "ctrl+c":
		return m, tea.Quit

	case "b":
		return m, m.startBuild(m.detailUnit)

	case "d":
		logPath := filepath.Join(m.projectDir, "build", m.detailUnit, "build.log")
		c := exec.Command("claude", fmt.Sprintf("diagnose %s", logPath))
		c.Dir = m.projectDir
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})

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

	// Unit list
	for _, i := range visible {
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

	// Search bar or help bar
	b.WriteString("\n")
	if m.searching {
		b.WriteString(fmt.Sprintf("  /%s▌", m.searchText))
	} else {
		b.WriteString(helpStyle.Render("  b build  e edit  d diagnose  l log  c clean  / search  q quit"))
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

	// Top half: executor output (dep progress)
	b.WriteString(headerStyle.Render("  BUILD OUTPUT"))
	b.WriteString("\n")
	if len(m.outputLines) == 0 {
		b.WriteString(dimStyle.Render("  (no output yet)"))
		b.WriteString("\n")
	} else {
		for _, line := range m.outputLines {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Separator
	b.WriteString("\n")

	// Bottom half: build log (compile output)
	b.WriteString(headerStyle.Render("  BUILD LOG"))
	b.WriteString("\n")
	if len(m.logLines) == 0 {
		b.WriteString(dimStyle.Render("  (no build log)"))
		b.WriteString("\n")
	} else {
		for _, line := range m.logLines {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  esc back  b build  d diagnose  l log"))
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
		return dimStyle.Render("● cached")
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

	proj := m.proj
	projectDir := m.projectDir
	unitName := name

	// Write executor output to a log file so detail view can tail it
	outputPath := filepath.Join(projectDir, "build", unitName, ".output.log")
	build.EnsureDir(filepath.Dir(outputPath))

	return func() tea.Msg {
		f, err := os.Create(outputPath)
		if err != nil {
			return buildDoneMsg{unit: unitName, err: err}
		}
		defer f.Close()

		err = build.BuildUnits(proj, []string{unitName}, build.Options{
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
	half := (m.height - 6) / 2
	if half < 5 {
		half = 10
	}
	outputPath := filepath.Join(m.projectDir, "build", m.detailUnit, ".output.log")
	m.outputLines = readFileTail(outputPath, half)
	logPath := filepath.Join(m.projectDir, "build", m.detailUnit, "build.log")
	m.logLines = readFileTail(logPath, half)
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

func readFileTail(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
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
