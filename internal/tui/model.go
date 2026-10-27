// Package tui is nauka's screen: a section's list on the left (containers,
// compose projects, images, volumes, networks, disk use), the selected
// one's details on the right.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theyashsawarkar/nauka/internal/dock"
	"github.com/Theyashsawarkar/nauka/internal/ui"
)

const (
	secContainers = iota
	secCompose
	secImages
	secVolumes
	secNetworks
	secSystem
	nSecs
)

var secNames = []string{"Containers", "Compose", "Images", "Volumes", "Networks", "System"}
var secKeys = []string{"containers", "compose", "images", "volumes", "networks", "system"}

const (
	subOverview = iota
	subLogs
	subStats
	subEnv
	subInspect
	subFiles
)

var subNames = []string{"Overview", "Logs", "Stats", "Env", "Inspect", "Files"}

// Options set how the screen runs.
type Options struct {
	PopupKey string // in a tmux popup: this key closes it too
}

type popup int

const (
	popNone popup = iota
	popHelp
	popConfirm
	popInput
	popPick
)

type confirm struct {
	title, yes, color string
	body              []string
	act               tea.Cmd
}

type inputBox struct {
	title, hint string
	in          textinput.Model
	act         func(string) tea.Cmd
}

type pickBox struct {
	title string
	opts  [][2]string // label, note
	sel   int
	act   func(int) tea.Cmd
}

// place is where Enter jumped from, for Backspace.
type place struct{ sec, sel, focus, sub int }

// link is something the details pane points at: Enter jumps to it.
type link struct {
	sec int
	key string // name or ID in that section
}

// Model is the screen.
type Model struct {
	opt   Options
	st    ui.Styles
	stamp string
	w, h  int

	sec   int
	sel   [nSecs]int
	top   [nSecs]int
	focus int // 0 the list, 1 the details
	sub   int
	dsel  int // the selected link, file or line in the details
	back  []place

	snap      dock.Snapshot
	loaded    bool
	loading   bool
	err       string
	stats     map[string]dock.Stats
	statsBusy bool
	usage     []dock.Usage
	volSize   map[string]string
	usageAt   time.Time
	usageBusy bool
	version   string

	logs     map[string][]string
	logsBusy bool
	imgs     map[string]dock.ImageDetails
	layers   bool
	reveal   bool
	fileCtr  string
	fileDir  string
	files    []dock.File
	fileErr  string
	compose  map[string][]string // compose file lines, by project
	showYAML bool

	filter    textinput.Model
	filtering bool

	pop     popup
	conf    *confirm
	input   *inputBox
	pick    *pickBox
	helpTop int

	dw int // the details pane's inner width, set while drawing

	msg    string
	msgErr bool
	msgID  int
	busy   string
}

// New makes the screen.
func New(opt Options) Model {
	f := textinput.New()
	f.Prompt = "/ "
	f.Placeholder = "name, image, project…"
	return Model{opt: opt, st: ui.New(ui.LoadTheme("nauka")), stamp: ui.ThemeStamp(), filter: f,
		stats: map[string]dock.Stats{}, volSize: map[string]string{}, logs: map[string][]string{},
		imgs: map[string]dock.ImageDetails{}, compose: map[string][]string{}, fileDir: "/"}
}

// ------------------------------------------------------------- messages

type snapMsg struct {
	s   dock.Snapshot
	err error
}
type statsMsg map[string]dock.Stats
type usageMsg struct {
	u       []dock.Usage
	sizes   map[string]string
	version string
}
type logsMsg struct {
	id    string
	lines []string
}
type imgMsg struct {
	id string
	d  dock.ImageDetails
}
type filesMsg struct {
	id, dir string
	files   []dock.File
	err     error
}
type composeMsg struct {
	project string
	lines   []string
}
type tickMsg struct{}
type doneMsg struct {
	text string
	err  error
}
type clearMsg struct{ id int }

func (m Model) Init() tea.Cmd { return tea.Batch(m.load(), m.loadUsage(), tick()) }

func tick() tea.Cmd { return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }

func (m *Model) load() tea.Cmd {
	if m.loading {
		return nil
	}
	m.loading = true
	return func() tea.Msg {
		s, err := dock.Load(context.Background())
		return snapMsg{s, err}
	}
}

func (m *Model) loadStats() tea.Cmd {
	if m.statsBusy || m.sec != secContainers {
		return nil
	}
	m.statsBusy = true
	return func() tea.Msg {
		st, _ := dock.LoadStats(context.Background())
		return statsMsg(st)
	}
}

func (m *Model) loadUsage() tea.Cmd {
	if m.usageBusy {
		return nil
	}
	m.usageBusy = true
	return func() tea.Msg {
		ctx := context.Background()
		u, sizes, _ := dock.LoadUsage(ctx)
		return usageMsg{u, sizes, dock.Version(ctx)}
	}
}

// loadDetails fetches what the details pane shows that the snapshot
// doesn't have: logs, an image's layers, a folder's files, the compose
// file.
func (m *Model) loadDetails() tea.Cmd {
	switch m.sec {
	case secContainers:
		c := m.curContainer()
		if c == nil {
			return nil
		}
		switch m.sub {
		case subLogs:
			if m.logsBusy {
				return nil
			}
			m.logsBusy = true
			id := c.ID
			return func() tea.Msg {
				l, _ := dock.Logs(context.Background(), id, 500)
				return logsMsg{id, l}
			}
		case subFiles:
			if m.fileCtr != c.ID {
				m.fileCtr, m.fileDir, m.files, m.fileErr = c.ID, "/", nil, ""
			}
			if !c.Running() {
				m.files, m.fileErr = nil, "start the container to see its files (s)"
				return nil
			}
			return m.listFiles(c.ID, m.fileDir)
		}
	case secImages:
		im := m.curImage()
		if im == nil {
			return nil
		}
		if _, ok := m.imgs[im.ID]; ok {
			return nil
		}
		id := im.ID
		return func() tea.Msg {
			d, _ := dock.InspectImage(context.Background(), id)
			return imgMsg{id, d}
		}
	case secCompose:
		p := m.curProject()
		if p == nil || len(p.ConfigFiles) == 0 {
			return nil
		}
		if _, ok := m.compose[p.Name]; ok {
			return nil
		}
		name, file := p.Name, p.ConfigFiles[0]
		return func() tea.Msg {
			b, err := os.ReadFile(file)
			if err != nil {
				return composeMsg{name, []string{"couldn't read " + file + ": " + err.Error()}}
			}
			return composeMsg{name, strings.Split(strings.TrimRight(string(b), "\n"), "\n")}
		}
	}
	return nil
}

func (m *Model) listFiles(id, dir string) tea.Cmd {
	return func() tea.Msg {
		fs, err := dock.Files(context.Background(), id, dir)
		return filesMsg{id, dir, fs, err}
	}
}

// ------------------------------------------------------------- update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case snapMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		first := !m.loaded
		m.snap, m.loaded, m.err = msg.s, true, ""
		m.clamp()
		if first {
			return m, tea.Batch(m.loadDetails(), m.loadStats())
		}
		return m, nil
	case statsMsg:
		m.statsBusy = false
		if msg != nil {
			m.stats = msg
		}
		return m, nil
	case usageMsg:
		m.usageBusy, m.usageAt = false, time.Now()
		m.usage, m.version = msg.u, msg.version
		if msg.sizes != nil {
			m.volSize = msg.sizes
		}
		return m, nil
	case logsMsg:
		m.logsBusy = false
		m.logs[msg.id] = msg.lines
		return m, nil
	case imgMsg:
		m.imgs[msg.id] = msg.d
		return m, nil
	case filesMsg:
		if msg.id == m.fileCtr {
			m.fileDir, m.files, m.fileErr = msg.dir, msg.files, ""
			if msg.err != nil {
				m.files, m.fileErr = nil, msg.err.Error()
			}
			m.dsel = 0
		}
		return m, nil
	case composeMsg:
		m.compose[msg.project] = msg.lines
		return m, nil
	case tickMsg:
		if s := ui.ThemeStamp(); s != m.stamp {
			m.stamp, m.st = s, ui.New(ui.LoadTheme("nauka"))
		}
		cmds := []tea.Cmd{tick(), m.load(), m.loadStats()}
		if time.Since(m.usageAt) > 30*time.Second {
			cmds = append(cmds, m.loadUsage())
		}
		if m.sec == secContainers && m.sub == subLogs {
			cmds = append(cmds, m.loadDetails())
		}
		return m, tea.Batch(cmds...)
	case doneMsg:
		m.busy = ""
		cmd := m.say(msg.text, msg.err)
		m.usageAt = time.Time{}
		return m, tea.Batch(cmd, m.load(), m.loadUsage(), m.loadDetails())
	case askMsg:
		m.ask(msg.title, msg.body, "prune", "", m.docker("pruning", "pruned", msg.args...))
		return m, nil
	case clearMsg:
		if msg.id == m.msgID {
			m.msg = ""
		}
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) say(text string, err error) tea.Cmd {
	m.msgErr = err != nil
	if err != nil {
		text = err.Error()
	}
	if text == "" {
		return nil
	}
	m.msg = text
	m.msgID++
	id := m.msgID
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return clearMsg{id} })
}

// ------------------------------------------------------------- rows

func (m Model) match(fields ...string) bool {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if q == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func (m Model) containers() []*dock.Container {
	var out []*dock.Container
	for i := range m.snap.Containers {
		c := &m.snap.Containers[i]
		if m.match(c.Name, c.Image, c.Project, c.Service, c.State) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) projects() []*dock.Project {
	var out []*dock.Project
	for i := range m.snap.Projects {
		if m.match(m.snap.Projects[i].Name, m.snap.Projects[i].WorkDir) {
			out = append(out, &m.snap.Projects[i])
		}
	}
	return out
}

func (m Model) images() []*dock.Image {
	var out []*dock.Image
	for i := range m.snap.Images {
		im := &m.snap.Images[i]
		if m.match(im.Repo, im.Tag, im.ID) {
			out = append(out, im)
		}
	}
	return out
}

func (m Model) volumes() []*dock.Volume {
	var out []*dock.Volume
	for i := range m.snap.Volumes {
		if m.match(m.snap.Volumes[i].Name) {
			out = append(out, &m.snap.Volumes[i])
		}
	}
	return out
}

func (m Model) networks() []*dock.Network {
	var out []*dock.Network
	for i := range m.snap.Networks {
		if m.match(m.snap.Networks[i].Name, m.snap.Networks[i].Driver) {
			out = append(out, &m.snap.Networks[i])
		}
	}
	return out
}

func (m Model) count(sec int) int {
	switch sec {
	case secContainers:
		return len(m.containers())
	case secCompose:
		return len(m.projects())
	case secImages:
		return len(m.images())
	case secVolumes:
		return len(m.volumes())
	case secNetworks:
		return len(m.networks())
	case secSystem:
		return len(m.usage)
	}
	return 0
}

func pick[T any](xs []*T, i int) *T {
	if i >= 0 && i < len(xs) {
		return xs[i]
	}
	return nil
}

func (m Model) curContainer() *dock.Container { return pick(m.containers(), m.sel[secContainers]) }
func (m Model) curProject() *dock.Project     { return pick(m.projects(), m.sel[secCompose]) }
func (m Model) curImage() *dock.Image         { return pick(m.images(), m.sel[secImages]) }
func (m Model) curVolume() *dock.Volume       { return pick(m.volumes(), m.sel[secVolumes]) }
func (m Model) curNetwork() *dock.Network     { return pick(m.networks(), m.sel[secNetworks]) }

func (m *Model) clamp() {
	for s := 0; s < nSecs; s++ {
		n := m.count(s)
		m.sel[s] = max(0, min(m.sel[s], n-1))
	}
}

// containersUsing are the containers with a volume mounted, or made from
// an image.
func (m Model) containersUsing(volume, imageID string) []*dock.Container {
	var out []*dock.Container
	for i := range m.snap.Containers {
		c := &m.snap.Containers[i]
		if imageID != "" && c.ImageID == imageID {
			out = append(out, c)
			continue
		}
		for _, mt := range c.Mounts {
			if volume != "" && mt.Name == volume {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func (m Model) containerByName(name string) *dock.Container {
	for i := range m.snap.Containers {
		if m.snap.Containers[i].Name == name || m.snap.Containers[i].ID == name {
			return &m.snap.Containers[i]
		}
	}
	return nil
}

// links are what the details pane points at, in order.
func (m Model) links() []link {
	var out []link
	switch m.sec {
	case secContainers:
		c := m.curContainer()
		if c == nil || m.sub != subOverview {
			return nil
		}
		out = append(out, link{secImages, c.ImageID})
		for _, mt := range c.Mounts {
			if mt.Type == "volume" {
				out = append(out, link{secVolumes, mt.Name})
			}
		}
		for _, n := range c.Networks {
			out = append(out, link{secNetworks, n.Network})
		}
	case secCompose:
		if p := m.curProject(); p != nil {
			for _, c := range m.snap.Containers {
				if c.Project == p.Name {
					out = append(out, link{secContainers, c.Name})
				}
			}
		}
	case secImages:
		if im := m.curImage(); im != nil {
			for _, c := range m.containersUsing("", im.ID) {
				out = append(out, link{secContainers, c.Name})
			}
		}
	case secVolumes:
		if v := m.curVolume(); v != nil {
			for _, c := range m.containersUsing(v.Name, "") {
				out = append(out, link{secContainers, c.Name})
			}
		}
	case secNetworks:
		if n := m.curNetwork(); n != nil {
			for _, c := range n.Containers {
				out = append(out, link{secContainers, c.Name})
			}
		}
	}
	return out
}

// jump goes to a link's row in its section, remembering where it was.
func (m *Model) jump(l link) bool {
	idx := -1
	switch l.sec {
	case secContainers:
		for i, c := range m.containers() {
			if c.Name == l.key || c.ID == l.key {
				idx = i
			}
		}
	case secImages:
		for i, im := range m.images() {
			if im.ID == l.key {
				idx = i
			}
		}
	case secVolumes:
		for i, v := range m.volumes() {
			if v.Name == l.key {
				idx = i
			}
		}
	case secNetworks:
		for i, n := range m.networks() {
			if n.Name == l.key {
				idx = i
			}
		}
	}
	if idx < 0 {
		return false
	}
	m.back = append(m.back, place{m.sec, m.sel[m.sec], m.focus, m.sub})
	m.sec, m.sel[l.sec], m.focus, m.dsel, m.sub = l.sec, idx, 0, 0, subOverview
	return true
}

// ------------------------------------------------------------- keys

func (m Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch m.pop {
	case popHelp:
		switch s {
		case "j", "down":
			m.helpTop++
		case "k", "up":
			m.helpTop = max(0, m.helpTop-1)
		default:
			m.pop = popNone
		}
		return m, nil
	case popConfirm:
		c := m.conf
		m.pop, m.conf = popNone, nil
		if s == "y" || s == "Y" {
			return m, c.act
		}
		return m, nil
	case popInput:
		switch s {
		case "esc", "ctrl+c":
			m.pop, m.input = popNone, nil
			return m, nil
		case "enter":
			v := strings.TrimSpace(m.input.in.Value())
			act := m.input.act
			m.pop, m.input = popNone, nil
			if v == "" {
				return m, nil
			}
			return m, act(v)
		}
		var cmd tea.Cmd
		m.input.in, cmd = m.input.in.Update(k)
		return m, cmd
	case popPick:
		p := m.pick
		switch s {
		case "esc", "q", "ctrl+c":
			m.pop, m.pick = popNone, nil
		case "j", "down":
			p.sel = min(len(p.opts)-1, p.sel+1)
		case "k", "up":
			p.sel = max(0, p.sel-1)
		case "enter":
			m.pop, m.pick = popNone, nil
			return m, p.act(p.sel)
		default:
			if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(p.opts) {
				m.pop, m.pick = popNone, nil
				return m, p.act(n - 1)
			}
		}
		return m, nil
	}
	if m.filtering {
		switch s {
		case "esc":
			m.filtering = false
			m.filter.SetValue("")
			m.filter.Blur()
			m.clamp()
		case "enter":
			m.filtering = false
			m.filter.Blur()
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(k)
			m.sel[m.sec] = 0
			m.clamp()
			return m, tea.Batch(cmd, m.loadDetails())
		}
		return m, nil
	}
	if m.opt.PopupKey != "" && s == m.opt.PopupKey {
		return m, tea.Quit
	}
	switch s {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		switch {
		case m.focus == 1:
			m.focus = 0
		case m.filter.Value() != "":
			m.filter.SetValue("")
			m.clamp()
		default:
			return m, tea.Quit
		}
		return m, nil
	case "?":
		m.pop, m.helpTop = popHelp, 0
		return m, nil
	case "/":
		m.filtering, m.focus = true, 0
		return m, m.filter.Focus()
	case "ctrl+r":
		m.usageAt = time.Time{}
		return m, tea.Batch(m.load(), m.loadUsage(), m.loadDetails(), m.say("reloading…", nil))
	case "tab":
		return m.setSec((m.sec + 1) % nSecs)
	case "shift+tab":
		return m.setSec((m.sec + nSecs - 1) % nSecs)
	case "backspace":
		if m.focus == 1 && m.sec == secContainers && m.sub == subFiles && m.fileDir != "/" {
			c := m.curContainer()
			if c != nil {
				return m, m.listFiles(c.ID, path.Dir(m.fileDir))
			}
		}
		if n := len(m.back); n > 0 {
			p := m.back[n-1]
			m.back = m.back[:n-1]
			m.sec, m.sel[p.sec], m.focus, m.sub, m.dsel = p.sec, p.sel, p.focus, p.sub, 0
			return m, m.loadDetails()
		}
		return m, nil
	}
	if m.sec == secContainers {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(subNames) {
			m.sub, m.dsel = n-1, 0
			if m.sub == subLogs {
				m.dsel = 1 << 30 // the newest line
			}
			return m, m.loadDetails()
		}
	}
	if m.focus == 1 {
		if mm, cmd, ok := m.detailKey(s); ok {
			return mm, cmd
		}
	} else {
		switch s {
		case "j", "down":
			return m.moveSel(1)
		case "k", "up":
			return m.moveSel(-1)
		case "home", "g":
			return m.moveSel(-1 << 20)
		case "end", "G":
			return m.moveSel(1 << 20)
		case "ctrl+d", "pgdown":
			return m.moveSel(max(1, m.h/3))
		case "ctrl+u", "pgup":
			return m.moveSel(-max(1, m.h/3))
		case "enter":
			m.focus, m.dsel = 1, 0
			return m, nil
		}
	}
	switch m.sec {
	case secContainers:
		return m.containerKey(s)
	case secCompose:
		return m.composeKey(s)
	case secImages:
		return m.imageKey(s)
	case secVolumes:
		return m.volumeKey(s)
	case secNetworks:
		return m.networkKey(s)
	case secSystem:
		if s == "P" {
			return m.pruneMenu()
		}
	}
	return m, nil
}

func (m Model) setSec(s int) (tea.Model, tea.Cmd) {
	m.sec, m.focus, m.dsel = s, 0, 0
	m.clamp()
	return m, tea.Batch(m.loadDetails(), m.loadStats())
}

func (m Model) moveSel(d int) (tea.Model, tea.Cmd) {
	n := m.count(m.sec)
	m.sel[m.sec] = max(0, min(n-1, m.sel[m.sec]+d))
	m.dsel = 0
	return m, m.loadDetails()
}

// detailKey handles keys while the details pane has focus; ok false
// passes the key on to the section's actions.
func (m Model) detailKey(s string) (tea.Model, tea.Cmd, bool) {
	limit := m.detailRows()
	m.dsel = min(m.dsel, max(0, limit-1))
	switch s {
	case "j", "down":
		m.dsel = min(max(0, limit-1), m.dsel+1)
		return m, nil, true
	case "k", "up":
		m.dsel = max(0, m.dsel-1)
		return m, nil, true
	case "ctrl+d", "pgdown":
		m.dsel = min(max(0, limit-1), m.dsel+m.h/3)
		return m, nil, true
	case "ctrl+u", "pgup":
		m.dsel = max(0, m.dsel-m.h/3)
		return m, nil, true
	case "home", "g":
		m.dsel = 0
		return m, nil, true
	case "end", "G":
		m.dsel = max(0, limit-1)
		return m, nil, true
	case "enter":
		if m.sec == secContainers && m.sub == subFiles {
			c := m.curContainer()
			if c != nil && m.dsel < len(m.files) && (m.files[m.dsel].Dir || m.files[m.dsel].Link) {
				return m, m.listFiles(c.ID, path.Join(m.fileDir, m.files[m.dsel].Name)), true
			}
			return m, nil, true
		}
		ls := m.links()
		if m.dsel < len(ls) && m.jump(ls[m.dsel]) {
			return m, m.loadDetails(), true
		}
		return m, nil, true
	}
	return m, nil, false
}

// detailRows is how many things j/k move through in the details pane.
func (m Model) detailRows() int {
	if m.sec == secContainers {
		c := m.curContainer()
		if c == nil {
			return 0
		}
		switch m.sub {
		case subLogs:
			return len(m.logs[c.ID])
		case subEnv:
			return len(c.Env)
		case subInspect:
			return len(m.inspectLines(c))
		case subFiles:
			return len(m.files)
		}
	}
	if m.sec == secCompose && m.showYAML {
		if p := m.curProject(); p != nil {
			return len(m.compose[p.Name])
		}
	}
	if m.sec == secImages && m.layers {
		if im := m.curImage(); im != nil {
			return len(m.imgs[im.ID].Layers)
		}
	}
	return len(m.links())
}

func (m Model) inspectLines(c *dock.Container) []string {
	var b strings.Builder
	var v any
	if json.Unmarshal(c.Raw, &v) != nil {
		return nil
	}
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
}

// ------------------------------------------------------------- actions

// docker runs a docker command in the background, saying what it does.
func (m *Model) docker(doing, done string, args ...string) tea.Cmd {
	m.busy = doing
	return tea.Batch(m.say(doing+"…", nil), func() tea.Msg {
		if _, err := dock.Run(context.Background(), args...); err != nil {
			return doneMsg{err: fmt.Errorf("%s: %v", doing, err)}
		}
		return doneMsg{text: done}
	})
}

// interactive runs a command in the terminal (nauka steps aside), then
// comes back. pause waits for Enter so its output can be read.
func interactive(pause bool, argv ...string) tea.Cmd {
	script := shellJoin(argv)
	if pause {
		script += `; s=$?; printf '\n\033[2m(press Enter to go back to nauka)\033[0m'; read -r _; exit $s`
	}
	c := exec.Command("sh", "-c", script)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 130 {
				return doneMsg{}
			}
			return doneMsg{err: fmt.Errorf("%s: %v", argv[0]+" "+argv[1], err)}
		}
		return doneMsg{}
	})
}

var safeArg = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

func shellJoin(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		if safeArg.MatchString(a) {
			q[i] = a
		} else {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(q, " ")
}

func (m *Model) ask(title string, body []string, yes, color string, act tea.Cmd) {
	if color == "" {
		color = m.st.T.Red
	}
	m.conf = &confirm{title: title, body: body, yes: yes, color: color, act: act}
	m.pop = popConfirm
}

func (m *Model) prompt(title, hint, value string, act func(string) tea.Cmd) tea.Cmd {
	in := textinput.New()
	in.Prompt = "› "
	in.SetValue(value)
	in.CursorEnd()
	m.input = &inputBox{title: title, hint: hint, in: in, act: act}
	m.pop = popInput
	return m.input.in.Focus()
}

func (m Model) containerKey(s string) (tea.Model, tea.Cmd) {
	c := m.curContainer()
	if c == nil {
		return m, nil
	}
	id, name := c.ID, c.Name
	switch s {
	case "s":
		if c.Running() {
			return m, m.docker("stopping "+name, "stopped "+name, "stop", id)
		}
		return m, m.docker("starting "+name, "started "+name, "start", id)
	case "r":
		return m, m.docker("restarting "+name, "restarted "+name, "restart", id)
	case "p":
		if c.State == "paused" {
			return m, m.docker("unpausing "+name, "unpaused "+name, "unpause", id)
		}
		if c.State != "running" {
			return m, m.say(name+" isn't running", nil)
		}
		return m, m.docker("pausing "+name, "paused "+name, "pause", id)
	case "e":
		if c.State != "running" {
			return m, m.say(name+" isn't running: s starts it", nil)
		}
		return m, interactive(false, "docker", "exec", "-it", id, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	case "l":
		return m, interactive(false, "docker", "logs", "-f", "--tail", "300", id)
	case "o":
		for _, p := range c.Published {
			if p.Proto == "tcp" {
				return m, openURL(fmt.Sprintf("http://localhost:%d", p.HostPort))
			}
		}
		return m, m.say(name+" publishes no port", nil)
	case "d":
		args := []string{"rm", id}
		body := []string{c.Image + " · " + c.Status}
		if c.Running() {
			args = []string{"rm", "-f", id}
			body = append(body, "It's running: it is stopped first.")
		}
		var named []string
		for _, mt := range c.Mounts {
			if mt.Type == "volume" {
				named = append(named, mt.Name)
			}
		}
		if len(named) > 0 {
			body = append(body, "Its volumes stay ("+strings.Join(shortNames(named), ", ")+"); remove them in Volumes.")
		}
		m.ask("remove "+name+"?", body, "remove the container", "", m.docker("removing "+name, "removed "+name, args...))
		return m, nil
	case "v":
		m.reveal = !m.reveal
		return m, nil
	case "c":
		if m.sub != subFiles || m.dsel >= len(m.files) {
			return m, m.say("pick a file in Files (6) first", nil)
		}
		f := m.files[m.dsel]
		src := path.Join(m.fileDir, f.Name)
		dl := filepath.Join(home(), "Downloads")
		_ = os.MkdirAll(dl, 0o755)
		return m, m.docker("copying "+src, "copied "+src+" to "+tilde(filepath.Join(dl, f.Name)), "cp", id+":"+src, filepath.Join(dl, f.Name))
	}
	return m, nil
}

func shortNames(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		if len(x) == 64 {
			x = x[:12] + "…"
		}
		out[i] = x
	}
	return out
}

func (m Model) composeArgs(p *dock.Project, args ...string) []string {
	base := []string{"docker", "compose", "-p", p.Name}
	if p.WorkDir != "" {
		base = append(base, "--project-directory", p.WorkDir)
	}
	for _, f := range p.ConfigFiles {
		base = append(base, "-f", f)
	}
	return append(base, args...)
}

func (m Model) composeKey(s string) (tea.Model, tea.Cmd) {
	p := m.curProject()
	if p == nil {
		return m, nil
	}
	switch s {
	case "u":
		return m, interactive(true, m.composeArgs(p, "up", "-d")...)
	case "d":
		m.ask("take "+p.Name+" down?", []string{
			fmt.Sprintf("docker compose down: its %d containers are stopped and removed, and its networks.", p.All),
			"Volumes stay (their data is kept); u brings it back up."}, "take it down", m.st.T.Peach,
			interactive(true, m.composeArgs(p, "down")...))
		return m, nil
	case "r":
		return m, interactive(true, m.composeArgs(p, "restart")...)
	case "b":
		return m, interactive(true, m.composeArgs(p, "build")...)
	case "p":
		return m, interactive(true, m.composeArgs(p, "pull")...)
	case "l":
		return m, interactive(false, m.composeArgs(p, "logs", "-f", "--tail", "100")...)
	case "e":
		if len(p.ConfigFiles) == 0 {
			return m, m.say("its compose file isn't known", nil)
		}
		ed := os.Getenv("VISUAL")
		if ed == "" {
			ed = os.Getenv("EDITOR")
		}
		if ed == "" {
			ed = "vi"
		}
		delete(m.compose, p.Name)
		return m, interactive(false, ed, p.ConfigFiles[0])
	case "y":
		m.showYAML = !m.showYAML
		m.dsel = 0
		return m, nil
	}
	return m, nil
}

func (m Model) imageKey(s string) (tea.Model, tea.Cmd) {
	im := m.curImage()
	if im == nil {
		return m, nil
	}
	ref := im.Ref()
	switch s {
	case "r":
		name := strings.ReplaceAll(strings.ReplaceAll(imageBase(im.Repo), "/", "-"), ":", "-")
		cmd := m.prompt("run "+ref, "docker run options before the image; Enter runs it", "-d --name "+name+" ", func(v string) tea.Cmd {
			args := append([]string{"docker", "run"}, strings.Fields(v)...)
			args = append(args, ref)
			return interactive(true, args...)
		})
		return m, cmd
	case "p":
		if im.Dangling {
			return m, m.say("an untagged image can't be pulled", nil)
		}
		return m, interactive(true, "docker", "pull", ref)
	case "H":
		m.layers = !m.layers
		m.dsel = 0
		return m, m.loadDetails()
	case "d":
		users := m.containersUsing("", im.ID)
		if len(users) > 0 {
			var names []string
			for _, c := range users {
				names = append(names, c.Name)
			}
			return m, m.say("used by "+strings.Join(names, ", ")+": remove those containers first", nil)
		}
		m.ask("remove "+ref+"?", []string{im.Size + " · made " + im.Created, "docker pull (or a build) brings it back."}, "remove the image", "",
			m.docker("removing "+ref, "removed "+ref, "rmi", im.ID))
		return m, nil
	case "P":
		var n int
		for _, x := range m.snap.Images {
			if x.Dangling && len(m.containersUsing("", x.ID)) == 0 {
				n++
			}
		}
		m.ask("remove dangling images?", []string{fmt.Sprintf("%d untagged images no container uses (docker image prune).", n)}, "remove them", "",
			m.docker("pruning images", "removed dangling images", "image", "prune", "-f"))
		return m, nil
	}
	return m, nil
}

func imageBase(repo string) string {
	return repo[strings.LastIndex(repo, "/")+1:]
}

func (m Model) volumeKey(s string) (tea.Model, tea.Cmd) {
	v := m.curVolume()
	switch s {
	case "n":
		return m, m.prompt("new volume", "its name", "", func(name string) tea.Cmd {
			return m.dockerCmd("creating "+name, "created volume "+name, "volume", "create", name)
		})
	case "P":
		var unused []string
		for _, x := range m.snap.Volumes {
			if len(m.containersUsing(x.Name, "")) == 0 {
				unused = append(unused, x.Name)
			}
		}
		body := []string{fmt.Sprintf("%d volumes no container uses (docker volume prune -a): their data is lost.", len(unused))}
		for i, u := range shortNames(unused) {
			if i == 6 {
				body = append(body, fmt.Sprintf("  … and %d more", len(unused)-6))
				break
			}
			body = append(body, "  "+u+"  "+m.volSize[unused[i]])
		}
		m.ask("remove unused volumes?", body, "remove them", "", m.docker("pruning volumes", "removed unused volumes", "volume", "prune", "-a", "-f"))
		return m, nil
	}
	if v == nil {
		return m, nil
	}
	name := v.Name
	switch s {
	case "f":
		return m, interactive(false, "docker", "run", "--rm", "-it", "-v", name+":/volume", "-w", "/volume", "alpine", "sh")
	case "d":
		users := m.containersUsing(name, "")
		if len(users) > 0 {
			var names []string
			for _, c := range users {
				names = append(names, c.Name)
			}
			return m, m.say("used by "+strings.Join(names, ", ")+": remove those containers first", nil)
		}
		size := m.volSize[name]
		if size == "" {
			size = "its"
		}
		m.ask("remove "+shortNames([]string{name})[0]+"?", []string{size + " of data, used by no container. This can't be undone."}, "remove the volume", "",
			m.docker("removing "+name, "removed volume "+shortNames([]string{name})[0], "volume", "rm", name))
		return m, nil
	}
	return m, nil
}

// dockerCmd is docker() for use inside a callback (it can't change m).
func (m Model) dockerCmd(doing, done string, args ...string) tea.Cmd {
	return func() tea.Msg {
		if _, err := dock.Run(context.Background(), args...); err != nil {
			return doneMsg{err: fmt.Errorf("%s: %v", doing, err)}
		}
		return doneMsg{text: done}
	}
}

func (m Model) networkKey(s string) (tea.Model, tea.Cmd) {
	n := m.curNetwork()
	switch s {
	case "n":
		return m, m.prompt("new network", "its name (a bridge network)", "", func(name string) tea.Cmd {
			return m.dockerCmd("creating "+name, "created network "+name, "network", "create", name)
		})
	case "P":
		m.ask("remove unused networks?", []string{"Networks no container is on (docker network prune). bridge, host and none stay."}, "remove them", m.st.T.Peach,
			m.docker("pruning networks", "removed unused networks", "network", "prune", "-f"))
		return m, nil
	}
	if n == nil {
		return m, nil
	}
	name := n.Name
	switch s {
	case "c":
		on := map[string]bool{}
		for _, c := range n.Containers {
			on[c.Name] = true
		}
		var opts [][2]string
		var ids []string
		for _, c := range m.snap.Containers {
			if !on[c.Name] {
				opts = append(opts, [2]string{c.Name, c.Status})
				ids = append(ids, c.ID)
			}
		}
		if len(opts) == 0 || n.Name == "host" || n.Name == "none" {
			return m, m.say("no container to connect", nil)
		}
		m.pick = &pickBox{title: "connect to " + name, opts: opts, act: func(i int) tea.Cmd {
			return m.dockerCmd("connecting", "connected "+opts[i][0]+" to "+name, "network", "connect", name, ids[i])
		}}
		m.pop = popPick
		return m, nil
	case "x":
		if len(n.Containers) == 0 {
			return m, m.say("no container is on "+name, nil)
		}
		var opts [][2]string
		for _, c := range n.Containers {
			opts = append(opts, [2]string{c.Name, c.IP})
		}
		m.pick = &pickBox{title: "disconnect from " + name, opts: opts, act: func(i int) tea.Cmd {
			return m.dockerCmd("disconnecting", "disconnected "+opts[i][0]+" from "+name, "network", "disconnect", name, opts[i][0])
		}}
		m.pop = popPick
		return m, nil
	case "d":
		if n.Builtin() {
			return m, m.say(name+" is Docker's own: it can't be removed", nil)
		}
		if len(n.Containers) > 0 {
			return m, m.say(fmt.Sprintf("%d containers are on %s: x disconnects them", len(n.Containers), name), nil)
		}
		m.ask("remove "+name+"?", []string{n.Driver + " · " + n.Subnet + " · no container on it."}, "remove the network", m.st.T.Peach,
			m.docker("removing "+name, "removed network "+name, "network", "rm", n.ID))
		return m, nil
	}
	return m, nil
}

func (m Model) pruneMenu() (tea.Model, tea.Cmd) {
	reclaim := map[string]string{}
	for _, u := range m.usage {
		reclaim[u.Type] = u.Reclaimable
	}
	type pr struct {
		label, note string
		args        []string
		warn        string
	}
	prs := []pr{
		{"stopped containers", reclaim["Containers"], []string{"container", "prune", "-f"}, "Every stopped container is removed."},
		{"dangling images", "untagged ones", []string{"image", "prune", "-f"}, "Untagged images no container uses are removed."},
		{"unused images", reclaim["Images"], []string{"image", "prune", "-a", "-f"}, "Every image no container uses is removed: the next run pulls or builds it again."},
		{"unused volumes", reclaim["Local Volumes"], []string{"volume", "prune", "-a", "-f"}, "Every volume no container uses is removed, with its data."},
		{"unused networks", "", []string{"network", "prune", "-f"}, "Networks no container is on are removed."},
		{"build cache", reclaim["Build Cache"], []string{"builder", "prune", "-f"}, "The build cache is cleared: the next build is slower."},
		{"everything unused", "all of the above", []string{"system", "prune", "-a", "--volumes", "-f"}, "Stopped containers, unused images, volumes (with their data), networks and the build cache are all removed."},
	}
	var opts [][2]string
	for _, p := range prs {
		opts = append(opts, [2]string{p.label, p.note})
	}
	m.pick = &pickBox{title: "prune what?", opts: opts, act: func(i int) tea.Cmd {
		p := prs[i]
		return func() tea.Msg { return askMsg{title: "prune " + p.label + "?", body: []string{p.warn}, args: p.args} }
	}}
	m.pop = popPick
	return m, nil
}

// askMsg opens a confirm from a callback.
type askMsg struct {
	title string
	body  []string
	args  []string
}

func openURL(u string) tea.Cmd {
	return func() tea.Msg {
		opener := "xdg-open"
		if b := os.Getenv("BROWSER"); b != "" {
			opener = b
		}
		cmd := exec.Command(opener, u)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return doneMsg{err: fmt.Errorf("couldn't open %s: %v", u, err)}
		}
		go func() { _ = cmd.Wait() }()
		return doneMsg{text: "opened " + u}
	}
}

func home() string { h, _ := os.UserHomeDir(); return h }

func tilde(p string) string {
	if h := home(); h != "" && strings.HasPrefix(p, h) {
		return "~" + p[len(h):]
	}
	return p
}
