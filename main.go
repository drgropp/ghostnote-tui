package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- THEME ----------

const (
	defaultAccent = "#4cc9f0" // Ghost Cyan
	dimText       = "#888888"
	errCol        = "#ff4b2b"
	textCol       = "#d1d1d1"
	selBg         = "#163b47"
	schemaID      = "ghostnote/v1"
)

// namedColors is the shared palette across the ecosystem (matches the web app).
var namedColors = map[string]string{
	"red": "#FF0000", "orange": "#FFA500", "yellow": "#FFFF00",
	"green": "#00ff00", "blue": "#0000FF", "purple": "#800080",
	"white": "#ffffff", "gray": "#888888", "grey": "#888888",
	"black": "#000000", "cyan": "#4cc9f0", "ghost": "#4cc9f0",
}

// resolveColor turns a name or hex string into a hex value, or "" if invalid.
func resolveColor(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if c, ok := namedColors[s]; ok {
		return c
	}
	if strings.HasPrefix(s, "#") && (len(s) == 4 || len(s) == 7) {
		return s
	}
	return ""
}

// theme holds the runtime-adjustable colors and the styles derived from them.
type theme struct {
	accent string
	cursor string

	titleStyle, fileStyle, statusKey, statusDim lipgloss.Style
	cmdStyle, msgOK, msgErr, edge               lipgloss.Style
	rowStyle, rowSelStyle, rowDateSty           lipgloss.Style
	taCursor, tiCursor                          lipgloss.Style
}

func newTheme(accent, cursor string) theme {
	if accent == "" {
		accent = defaultAccent
	}
	if cursor == "" {
		cursor = accent
	}
	t := theme{accent: accent, cursor: cursor}
	a := lipgloss.Color(accent)
	t.titleStyle = lipgloss.NewStyle().Foreground(a).Bold(true)
	t.fileStyle = lipgloss.NewStyle().Foreground(a)
	t.statusKey = lipgloss.NewStyle().Foreground(a).Bold(true)
	t.statusDim = lipgloss.NewStyle().Foreground(lipgloss.Color(dimText))
	t.cmdStyle = lipgloss.NewStyle().Foreground(a)
	t.msgOK = lipgloss.NewStyle().Foreground(a)
	t.msgErr = lipgloss.NewStyle().Foreground(lipgloss.Color(errCol))
	t.edge = lipgloss.NewStyle().Foreground(a)
	t.rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(textCol))
	t.rowSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color(selBg)).Bold(true)
	t.rowDateSty = lipgloss.NewStyle().Foreground(lipgloss.Color(dimText))
	t.taCursor = lipgloss.NewStyle().Background(lipgloss.Color(cursor))
	t.tiCursor = lipgloss.NewStyle().Foreground(lipgloss.Color(cursor))
	return t
}

// th is the active theme, rebuilt whenever colors change.
var th = newTheme(defaultAccent, defaultAccent)

// hueToHex converts an HSL hue (0-359, full sat, ~60% light) to a hex string.
func hueToHex(h int) string {
	// S=1.0, L=0.6
	c := 0.8 // chroma = (1-|2L-1|)*S = (1-0.2)*1
	hp := float64(h%360) / 60.0
	x := c * (1 - absf(modf(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := 0.6 - c/2
	to := func(v float64) int { return int((v + m) * 255) }
	return fmt.Sprintf("#%02x%02x%02x", to(r), to(g), to(b))
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
func modf(a, b float64) float64 {
	for a >= b {
		a -= b
	}
	return a
}

func rgbTick() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(time.Time) tea.Msg { return rgbTickMsg{} })
}

// spectrum is the curated palette the :ui rgb spectrum mode fades through.
var spectrum = []string{"#FF0000", "#FFA500", "#FFFF00", "#00ff00", "#0000FF", "#800080"}

func hexByte(s string, i int) int {
	var v int
	fmt.Sscanf(s[i:i+2], "%02x", &v)
	return v
}

// lerpHex blends two #rrggbb colors by t in [0,1].
func lerpHex(a, b string, t float64) string {
	ar, ag, ab := hexByte(a, 1), hexByte(a, 3), hexByte(a, 5)
	br, bg, bb := hexByte(b, 1), hexByte(b, 3), hexByte(b, 5)
	mix := func(x, y int) int { return int(float64(x) + (float64(y)-float64(x))*t) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// spectrumColor returns the blended color at a float step in [0, len(spectrum)).
func spectrumColor(step float64) string {
	n := len(spectrum)
	i := int(step) % n
	t := step - float64(int(step))
	return lerpHex(spectrum[i], spectrum[(i+1)%n], t)
}

// ---------- FORMAT (ghostnote/v1) ----------

type noteMeta struct {
	TextColor    string  `json:"textColor,omitempty"`
	TextSize     string  `json:"textSize,omitempty"`
	PenColor     string  `json:"penColor,omitempty"`
	PenWeight    float64 `json:"penWeight,omitempty"`
	EraserWeight float64 `json:"eraserWeight,omitempty"`
	BgAlpha      string  `json:"bgAlpha,omitempty"`
	TextAlpha    string  `json:"textAlpha,omitempty"`
}

type note struct {
	Schema  string   `json:"schema"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Text    string   `json:"text"`
	Drawing *string  `json:"drawing"` // web-only; preserved on round-trip
	Meta    noteMeta `json:"meta"`
	Created int64    `json:"created"`
	Updated int64    `json:"updated"`
}

func nowMS() int64 { return time.Now().UnixMilli() }

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	if s == "" {
		s = "note"
	}
	return s
}

// ---------- STORAGE ----------

func notesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".ghostnote", "notes")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func notePath(name string) string {
	return filepath.Join(notesDir(), slug(name)+".ghostnote.json")
}

// ---------- THEME CONFIG ----------

type themeConfig struct {
	Accent  string `json:"accent"`
	Cursor  string `json:"cursor"`
	Rgb     bool   `json:"rgb"`
	RgbMode string `json:"rgbMode"`
}

func themeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".ghostnote")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "tui-config.json")
}

func loadThemeConfig() themeConfig {
	tc := themeConfig{Accent: defaultAccent, Cursor: defaultAccent}
	data, err := os.ReadFile(themeConfigPath())
	if err != nil {
		return tc
	}
	_ = json.Unmarshal(data, &tc)
	if tc.Accent == "" {
		tc.Accent = defaultAccent
	}
	if tc.Cursor == "" {
		tc.Cursor = tc.Accent
	}
	return tc
}

func saveThemeConfig(tc themeConfig) {
	data, _ := json.MarshalIndent(tc, "", "  ")
	_ = os.WriteFile(themeConfigPath(), data, 0o644)
}

func saveNoteFile(n note) error {
	if n.Schema == "" {
		n.Schema = schemaID
	}
	if n.ID == "" {
		n.ID = newUUID()
	}
	n.Updated = nowMS()
	if n.Created == 0 {
		n.Created = n.Updated
	}
	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(notePath(n.Name), data, 0o644)
}

func loadNoteFile(name string) (note, error) {
	var n note
	data, err := os.ReadFile(notePath(name))
	if err != nil {
		return n, err
	}
	if err := json.Unmarshal(data, &n); err != nil {
		return n, err
	}
	return n, nil
}

func deleteNoteFile(name string) error {
	return os.Remove(notePath(name))
}

type noteIndex struct {
	Name    string
	Updated int64
}

func listNoteIndex() []noteIndex {
	entries, err := os.ReadDir(notesDir())
	if err != nil {
		return nil
	}
	var out []noteIndex
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ghostnote.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(notesDir(), e.Name()))
		if err != nil {
			continue
		}
		var n note
		if json.Unmarshal(data, &n) != nil {
			continue
		}
		name := n.Name
		if name == "" {
			name = strings.TrimSuffix(e.Name(), ".ghostnote.json")
		}
		out = append(out, noteIndex{Name: name, Updated: n.Updated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out
}

// ---------- MODEL ----------

type mode int

const (
	modeType mode = iota
	modeCommand
	modeList
)

type clearMsgMsg struct{ id int }
type rgbTickMsg struct{}

type model struct {
	area     textarea.Model
	cmd      textinput.Model
	mode     mode
	width    int
	height   int
	curName  string // current note's name ("" = unsaved)
	curID    string // current note's id (preserve on save)
	curMeta  noteMeta
	curDraw  *string
	created  int64
	message  string
	msgIsErr bool
	msgID    int

	// list panel state
	list    []noteIndex
	listSel int

	// rainbow state
	rgbOn   bool
	rgbMode string // "rainbow" or "spectrum"
	rgbHue  int
	rgbStep float64

	quitting bool
}

func initialModel() model {
	// load persisted theme before building any styles
	tc := loadThemeConfig()
	th = newTheme(tc.Accent, tc.Cursor)

	ta := textarea.New()
	ta.Placeholder = "begin"
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.Focus()
	// Keep Ctrl+V as an application-level clipboard fallback. Terminals that
	// handle paste themselves send the pasted text through bracketed paste;
	// terminals that forward Ctrl+V let the textarea read the system clipboard.
	ta.KeyMap.Paste.SetKeys("ctrl+v")
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(textCol))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(textCol))
	ta.Cursor.Style = th.taCursor

	ti := textinput.New()
	ti.Prompt = ":"
	ti.PromptStyle = th.cmdStyle
	ti.TextStyle = th.cmdStyle
	ti.Cursor.Style = th.tiCursor
	ti.CharLimit = 200
	ti.KeyMap.Paste.SetKeys("ctrl+v")

	return model{
		area:    ta,
		cmd:     ti,
		mode:    modeType,
		curName: "",
		rgbOn:   tc.Rgb,
		rgbMode: tc.RgbMode,
	}
}

func (m model) Init() tea.Cmd {
	if m.rgbOn {
		return tea.Batch(textarea.Blink, rgbTick())
	}
	return textarea.Blink
}

func (m model) fileLabel() string {
	if m.curName == "" {
		return "untitled"
	}
	return m.curName + ".md"
}

// ---------- UPDATE ----------

func (m model) flash(text string, isErr bool) (model, tea.Cmd) {
	m.message = text
	m.msgIsErr = isErr
	m.msgID++
	id := m.msgID
	return m, tea.Tick(time.Millisecond*1500, func(time.Time) tea.Msg {
		return clearMsgMsg{id: id}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case clearMsgMsg:
		if msg.id == m.msgID {
			m.message = ""
		}
		return m, nil

	case rgbTickMsg:
		if !m.rgbOn {
			return m, nil // stopped; let the loop die
		}
		var c string
		if m.rgbMode == "spectrum" {
			m.rgbStep = m.rgbStep + 0.04
			if m.rgbStep >= float64(len(spectrum)) {
				m.rgbStep -= float64(len(spectrum))
			}
			c = spectrumColor(m.rgbStep)
		} else {
			m.rgbHue = (m.rgbHue + 8) % 360
			c = hueToHex(m.rgbHue)
		}
		m = m.setTheme(c, c)
		return m, rgbTick()

	case tea.KeyMsg:
		switch m.mode {
		case modeType:
			return m.updateType(msg)
		case modeCommand:
			return m.updateCommand(msg)
		case modeList:
			return m.updateList(msg)
		}
	}

	// non-key messages pass to the active component
	var cmd tea.Cmd
	if m.mode == modeCommand {
		m.cmd, cmd = m.cmd.Update(msg)
	} else if m.mode == modeType {
		m.area, cmd = m.area.Update(msg)
	}
	return m, cmd
}

func (m model) updateType(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}
	if msg.String() == ":" {
		m.mode = modeCommand
		m.cmd.SetValue("")
		m.cmd.Focus()
		m.area.Blur()
		return m, textinput.Blink
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(msg)
	return m, cmd
}

func (m model) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.mode = modeType
		m.cmd.Blur()
		m.area.Focus()
		return m, textarea.Blink
	case tea.KeyEnter:
		val := strings.TrimPrefix(strings.TrimSpace(m.cmd.Value()), ":")
		m.cmd.SetValue("")
		m.cmd.Blur()
		m.mode = modeType
		m.area.Focus()
		nm, cmd := m.runCommand(val)
		return nm, tea.Batch(cmd, textarea.Blink)
	}
	var cmd tea.Cmd
	m.cmd, cmd = m.cmd.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.mode = modeType
		m.area.Focus()
		return m, textarea.Blink
	case tea.KeyUp:
		if m.listSel > 0 {
			m.listSel--
		}
		return m, nil
	case tea.KeyDown:
		if m.listSel < len(m.list)-1 {
			m.listSel++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.list) == 0 {
			m.mode = modeType
			m.area.Focus()
			return m, textarea.Blink
		}
		name := m.list[m.listSel].Name
		m.mode = modeType
		m.area.Focus()
		nm, cmd := m.openNote(name)
		return nm, tea.Batch(cmd, textarea.Blink)
	}
	switch msg.String() {
	case "k":
		if m.listSel > 0 {
			m.listSel--
		}
	case "j":
		if m.listSel < len(m.list)-1 {
			m.listSel++
		}
	case "x", "d":
		if len(m.list) > 0 {
			name := m.list[m.listSel].Name
			_ = deleteNoteFile(name)
			m.list = listNoteIndex()
			if m.listSel >= len(m.list) {
				m.listSel = len(m.list) - 1
			}
			if m.listSel < 0 {
				m.listSel = 0
			}
		}
	}
	return m, nil
}

func (m *model) layout() {
	innerW := m.width - 2
	innerH := m.height - 2 - 1
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	m.area.SetWidth(innerW)
	m.area.SetHeight(innerH)
	m.cmd.Width = innerW - 4
}

// ---------- COMMANDS ----------

func (m model) runCommand(val string) (model, tea.Cmd) {
	if val == "" {
		return m, nil
	}
	lower := strings.ToLower(val)
	fields := strings.Fields(val)
	arg := ""
	if len(fields) > 1 {
		arg = strings.Join(fields[1:], " ")
	}

	switch {
	case lower == "help" || lower == "?":
		return m.flash("save open notes new wipe rename copy search export import ui reset quit", false)
	case lower == "quit" || lower == "q" || lower == "exit":
		m.quitting = true
		return m, tea.Quit
	case lower == "wipe" || lower == "clear":
		m.area.SetValue("")
		return m.flash("wiped", false)
	case lower == "new":
		m.area.SetValue("")
		m.curName, m.curID, m.curDraw, m.created = "", "", nil, 0
		m.curMeta = noteMeta{}
		return m.flash("new note", false)
	case lower == "save":
		return m.saveNote(m.curName)
	case strings.HasPrefix(lower, "save "):
		return m.saveNote(arg)
	case lower == "open" || lower == "load":
		return m.flash("usage: :open [name]", true)
	case strings.HasPrefix(lower, "open ") || strings.HasPrefix(lower, "load "):
		return m.openNote(arg)
	case lower == "notes" || lower == "list" || lower == "ls":
		m.list = listNoteIndex()
		m.listSel = 0
		m.mode = modeList
		m.area.Blur()
		return m, nil
	case strings.HasPrefix(lower, "delete ") || strings.HasPrefix(lower, "del ") || strings.HasPrefix(lower, "rm "):
		return m.deleteNote(arg)
	case strings.HasPrefix(lower, "rename "):
		return m.renameNote(arg)
	case strings.HasPrefix(lower, "copy ") || strings.HasPrefix(lower, "duplicate "):
		return m.copyNote(arg)
	case lower == "search" || lower == "find":
		return m.flash("usage: :search [term]", true)
	case strings.HasPrefix(lower, "search ") || strings.HasPrefix(lower, "find "):
		return m.searchNotes(arg)
	case strings.HasPrefix(lower, "export "):
		return m.exportNote(fields[1:])
	case strings.HasPrefix(lower, "import "):
		return m.importNote(arg)
	case lower == "reset":
		return m.resetTheme()
	case lower == "ui" || lower == "color":
		return m.flash("usage: :ui [color] | :ui accent [c] | :ui cursor [c]", true)
	case strings.HasPrefix(lower, "ui "):
		return m.setUI(fields[1:])
	default:
		return m.flash("unknown command: "+val, true)
	}
}

// setTheme applies colors live without persisting (used by the rgb animation).
func (m model) setTheme(accent, cursor string) model {
	th = newTheme(accent, cursor)
	m.area.Cursor.Style = th.taCursor
	m.cmd.Cursor.Style = th.tiCursor
	m.cmd.PromptStyle = th.cmdStyle
	m.cmd.TextStyle = th.cmdStyle
	return m
}

// applyTheme applies colors AND persists them. Use for explicit user changes.
func (m model) applyTheme(accent, cursor string) model {
	m = m.setTheme(accent, cursor)
	saveThemeConfig(themeConfig{Accent: th.accent, Cursor: th.cursor})
	return m
}

func (m model) setUI(args []string) (model, tea.Cmd) {
	// rgb modes
	if len(args) >= 1 && strings.ToLower(args[0]) == "rgb" {
		sub := ""
		if len(args) >= 2 {
			sub = strings.ToLower(args[1])
		}
		if sub == "static" {
			m.rgbOn = false
			c := hueToHex(280) // fixed vivid hue
			m = m.applyTheme(c, c)
			saveThemeConfig(themeConfig{Accent: th.accent, Cursor: th.cursor, Rgb: false})
			return m.flash("ui: rgb static", false)
		}
		if sub == "spectrum" {
			m.rgbOn = true
			m.rgbMode = "spectrum"
			m.rgbStep = 0
			saveThemeConfig(themeConfig{Accent: th.accent, Cursor: th.cursor, Rgb: true, RgbMode: "spectrum"})
			return m, rgbTick()
		}
		// default: full rainbow
		if m.rgbOn && m.rgbMode != "spectrum" {
			return m.flash("rgb already on", false)
		}
		m.rgbOn = true
		m.rgbMode = "rainbow"
		m.rgbHue = 0
		saveThemeConfig(themeConfig{Accent: th.accent, Cursor: th.cursor, Rgb: true, RgbMode: "rainbow"})
		return m, rgbTick()
	}

	if len(args) == 1 {
		// :ui [color] -> both accent and cursor
		c := resolveColor(args[0])
		if c == "" {
			return m.flash("unknown color: "+args[0], true)
		}
		m.rgbOn = false
		m = m.applyTheme(c, c)
		return m.flash("ui: "+args[0], false)
	}
	if len(args) >= 2 {
		slot := strings.ToLower(args[0])
		c := resolveColor(args[1])
		if c == "" {
			return m.flash("unknown color: "+args[1], true)
		}
		switch slot {
		case "accent":
			m.rgbOn = false
			m = m.applyTheme(c, th.cursor)
			return m.flash("accent: "+args[1], false)
		case "cursor":
			m.rgbOn = false
			m = m.applyTheme(th.accent, c)
			return m.flash("cursor: "+args[1], false)
		default:
			return m.flash("slot must be accent or cursor", true)
		}
	}
	return m.flash("usage: :ui [color] | :ui accent [c] | :ui cursor [c] | :ui rgb", true)
}

func (m model) resetTheme() (model, tea.Cmd) {
	m.rgbOn = false
	m.rgbMode = ""
	m = m.applyTheme(defaultAccent, defaultAccent)
	saveThemeConfig(themeConfig{Accent: defaultAccent, Cursor: defaultAccent, Rgb: false})
	return m.flash("theme reset", false)
}

func (m model) renameNote(newName string) (model, tea.Cmd) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return m.flash("usage: :rename [newname]", true)
	}
	if m.curName == "" {
		return m.flash("save the note first", true)
	}
	old := m.curName
	// save under new name, then delete old file
	n := note{Schema: schemaID, ID: m.curID, Name: newName, Text: m.area.Value(),
		Drawing: m.curDraw, Meta: m.curMeta, Created: m.created}
	if err := saveNoteFile(n); err != nil {
		return m.flash("rename failed", true)
	}
	if slug(old) != slug(newName) {
		_ = deleteNoteFile(old)
	}
	m.curName = newName
	return m.flash("renamed: "+old+" → "+newName, false)
}

func (m model) copyNote(newName string) (model, tea.Cmd) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return m.flash("usage: :copy [newname]", true)
	}
	// new id, current content, new name
	n := note{Schema: schemaID, Name: newName, Text: m.area.Value(),
		Drawing: m.curDraw, Meta: m.curMeta}
	if err := saveNoteFile(n); err != nil {
		return m.flash("copy failed", true)
	}
	return m.flash("copied to: "+newName, false)
}

func (m model) searchNotes(term string) (model, tea.Cmd) {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return m.flash("usage: :search [term]", true)
	}
	var hits []string
	for _, it := range listNoteIndex() {
		matchName := strings.Contains(strings.ToLower(it.Name), term)
		matchBody := false
		if n, err := loadNoteFile(it.Name); err == nil {
			matchBody = strings.Contains(strings.ToLower(n.Text), term)
		}
		if matchName || matchBody {
			tag := it.Name
			if matchBody && !matchName {
				tag += " (in text)"
			}
			hits = append(hits, tag)
		}
	}
	if len(hits) == 0 {
		return m.flash("no matches for: "+term, false)
	}
	// show results in the list panel filtered
	m.list = nil
	for _, it := range listNoteIndex() {
		nameHit := strings.Contains(strings.ToLower(it.Name), term)
		bodyHit := false
		if n, err := loadNoteFile(it.Name); err == nil {
			bodyHit = strings.Contains(strings.ToLower(n.Text), term)
		}
		if nameHit || bodyHit {
			m.list = append(m.list, it)
		}
	}
	m.listSel = 0
	m.mode = modeList
	m.area.Blur()
	return m, nil
}

func (m model) exportNote(args []string) (model, tea.Cmd) {
	if len(args) == 0 {
		return m.flash("usage: :export [name] [path]", true)
	}
	name := args[0]
	n, err := loadNoteFile(name)
	if err != nil {
		return m.flash("not found: "+name, true)
	}
	path := slug(name) + ".ghostnote.json"
	if len(args) > 1 {
		path = args[1]
	}
	data, _ := json.MarshalIndent(n, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return m.flash("export failed", true)
	}
	return m.flash("exported: "+path, false)
}

func (m model) importNote(path string) (model, tea.Cmd) {
	path = strings.TrimSpace(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return m.flash("can't read: "+path, true)
	}
	var n note
	if err := json.Unmarshal(data, &n); err != nil {
		return m.flash("bad json", true)
	}
	if n.Schema == "" {
		n.Schema = schemaID
	}
	if n.Name == "" {
		return m.flash("note has no name", true)
	}
	if err := saveNoteFile(n); err != nil {
		return m.flash("import failed", true)
	}
	m.area.SetValue(n.Text)
	m.curName = n.Name
	m.curID = n.ID
	m.curMeta = n.Meta
	m.curDraw = n.Drawing
	m.created = n.Created
	return m.flash("imported: "+n.Name, false)
}

func (m model) saveNote(name string) (model, tea.Cmd) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = m.curName
	}
	if name == "" {
		name = "notes"
	}
	n := note{
		Schema:  schemaID,
		ID:      m.curID,
		Name:    name,
		Text:    m.area.Value(),
		Drawing: m.curDraw,
		Meta:    m.curMeta,
		Created: m.created,
	}
	if err := saveNoteFile(n); err != nil {
		return m.flash("save failed", true)
	}
	// reload to capture assigned id/timestamps
	if saved, err := loadNoteFile(name); err == nil {
		m.curID = saved.ID
		m.created = saved.Created
	}
	m.curName = name
	return m.flash("saved: "+m.fileLabel(), false)
}

func (m model) openNote(name string) (model, tea.Cmd) {
	name = strings.TrimSpace(name)
	n, err := loadNoteFile(name)
	if err != nil {
		return m.flash("not found: "+name, true)
	}
	m.area.SetValue(n.Text)
	m.curName = n.Name
	if m.curName == "" {
		m.curName = name
	}
	m.curID = n.ID
	m.curMeta = n.Meta
	m.curDraw = n.Drawing // preserve web drawing on round-trip
	m.created = n.Created
	hint := ""
	if n.Drawing != nil {
		hint = " (has drawing — preserved)"
	}
	return m.flash("opened: "+m.fileLabel()+hint, false)
}

func (m model) deleteNote(name string) (model, tea.Cmd) {
	name = strings.TrimSpace(name)
	if err := deleteNoteFile(name); err != nil {
		return m.flash("not found: "+name, true)
	}
	if slug(m.curName) == slug(name) {
		m.curName, m.curID, m.curDraw, m.created = "", "", nil, 0
		m.curMeta = noteMeta{}
	}
	return m.flash("deleted: "+name, false)
}

// ---------- VIEW ----------

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "loading..."
	}
	innerW := m.width - 2

	left := th.titleStyle.Render("[ GHOSTNOTE ]")
	right := th.fileStyle.Render(m.fileLabel())
	top := composeTopBorder(innerW, left, right)
	bottom := composeBottomBorder(innerW)

	var body string
	if m.mode == modeList {
		body = m.listView(innerW, m.height-2-1)
	} else {
		body = m.area.View()
	}

	var b strings.Builder
	b.WriteString(top + "\n")
	for _, line := range strings.Split(body, "\n") {
		b.WriteString(th.edge.Render("│") + padRight(line, innerW) + th.edge.Render("│") + "\n")
	}
	b.WriteString(bottom)

	return b.String() + "\n" + m.statusBar(innerW)
}

func (m model) listView(width, height int) string {
	var rows []string
	header := lipgloss.NewStyle().Foreground(lipgloss.Color(th.accent)).Bold(true).Render("  SAVED NOTES")
	rows = append(rows, header, "")
	if len(m.list) == 0 {
		rows = append(rows, th.rowDateSty.Render("  no saved notes yet — try :save [name]"))
	}
	for i, it := range m.list {
		name := it.Name
		date := fmtDate(it.Updated)
		nameW := width - len(date) - 6
		if nameW < 1 {
			nameW = 1
		}
		namePart := truncate(name, nameW)
		if i == m.listSel {
			line := fmt.Sprintf("› %-*s %s", nameW, namePart, date)
			rows = append(rows, th.rowSelStyle.Render(padRight(line, width)))
		} else {
			line := th.rowStyle.Render(fmt.Sprintf("  %-*s ", nameW, namePart)) + th.rowDateSty.Render(date)
			rows = append(rows, line)
		}
	}
	// pad to fill height
	for len(rows) < height {
		rows = append(rows, "")
	}
	if len(rows) > height {
		rows = rows[:height]
	}
	return strings.Join(rows, "\n")
}

func (m model) statusBar(width int) string {
	if m.mode == modeCommand {
		return th.statusKey.Render("[CMD]") + " " + m.cmd.View()
	}
	label := "[TYPE]"
	help := th.statusDim.Render(":") + th.statusKey.Render(" commands") + "   " +
		th.statusKey.Render("^V") + th.statusDim.Render(" paste") + "   " +
		th.statusKey.Render("^C") + th.statusDim.Render(" quit")
	if m.mode == modeList {
		label = "[NOTES]"
		help = th.statusKey.Render("↑↓/jk") + th.statusDim.Render(" move") + "  " +
			th.statusKey.Render("enter") + th.statusDim.Render(" open") + "  " +
			th.statusKey.Render("x") + th.statusDim.Render(" delete") + "  " +
			th.statusKey.Render("esc") + th.statusDim.Render(" back")
	}
	left := th.statusKey.Render(label) + "  " + help
	if m.message != "" && m.mode != modeList {
		msg := th.msgOK.Render(m.message)
		if m.msgIsErr {
			msg = th.msgErr.Render(m.message)
		}
		gap := width - lipgloss.Width(left) - lipgloss.Width(msg)
		if gap < 1 {
			gap = 1
		}
		return left + strings.Repeat(" ", gap) + msg
	}
	return left
}

func composeTopBorder(innerW int, left, right string) string {
	c := th.edge
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	fixed := 5 // ─ <left> ─ <fill> space <right> space ─
	fill := innerW - lw - rw - fixed
	if fill < 1 {
		fill = 1
	}
	inner := c.Render("─") + left + c.Render("─") +
		c.Render(strings.Repeat("─", fill)) +
		c.Render(" ") + right + c.Render(" ─")
	return c.Render("╭") + inner + c.Render("╮")
}

func composeBottomBorder(innerW int) string {
	return th.edge.Render("╰" + strings.Repeat("─", innerW) + "╯")
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}

func fmtDate(ts int64) string {
	if ts == 0 {
		return ""
	}
	t := time.UnixMilli(ts)
	return t.Format("01/02 15:04")
}

// ---------- MAIN ----------

func main() {
	// Do not enable Bubble Tea mouse reporting here. Leaving mouse reporting off
	// lets the terminal own drag-selection and its normal copy shortcuts.
	// Bubble Tea enables bracketed paste by default, while the input components
	// provide a Ctrl+V clipboard fallback when that key reaches the application.
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
