package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Theyashsawarkar/nauka/internal/dock"
	"github.com/Theyashsawarkar/nauka/internal/ui"
)

func (m Model) View() string {
	if m.w == 0 {
		return ""
	}
	st := m.st
	var tabs []string
	for i, n := range secNames {
		if i == secSystem {
			tabs = append(tabs, n)
			continue
		}
		tabs = append(tabs, fmt.Sprintf("%s %d", n, m.count(i)))
	}
	right := "nauka"
	if m.version != "" {
		right += " · docker " + m.version
	}
	running := 0
	for _, c := range m.snap.Containers {
		if c.State == "running" {
			running++
		}
	}
	right += fmt.Sprintf(" · %d running", running)
	if st.Nerd {
		right = "\uf308 " + right
	}
	rows := []string{st.Tabs(tabs, m.sec, right, m.w)}
	bh := m.h - 2
	if m.filtering || m.filter.Value() != "" {
		rows = append(rows, " "+m.filter.View())
		bh--
	}
	lw := max(40, m.w*52/100)
	if m.sec == secSystem {
		lw = max(40, m.w*55/100)
	}
	rw := m.w - lw
	m.dw = rw - 4
	ltitle, llines := m.list(lw-4, bh-2)
	dtitle, dlines := m.details(rw-4, bh-2)
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		st.Box(lw, bh, ltitle, llines, m.focus == 0, ""),
		st.Box(rw, bh, dtitle, dlines, m.focus == 1, ""))
	var box []string
	switch m.pop {
	case popHelp:
		box = m.helpBox(bh)
	case popConfirm:
		w := min(m.w-4, 76)
		var lines []string
		for _, b := range m.conf.body {
			lines = append(lines, ui.Wrap(b, w-4)...)
		}
		lines = append(lines, "", st.Key("y", m.conf.yes)+"   "+st.Key("n", "keep it"))
		box = st.Popup(m.conf.title, lines, w, m.conf.color)
	case popInput:
		w := min(m.w-4, 80)
		in := m.input.in
		in.Width = w - 8
		box = st.Popup(m.input.title, []string{st.Muted.Render(m.input.hint), "", in.View(), "", st.Muted.Render("enter runs it · esc cancels")}, w, "")
	case popPick:
		w := min(m.w-4, 72)
		var lines []string
		for i, o := range m.pick.opts {
			bar := "  "
			label := st.Bold.Render(o[0])
			if i == m.pick.sel {
				bar = st.Accent.Render("▌ ")
			}
			lines = append(lines, bar+st.Key(strconv.Itoa(i+1), "")+" "+label+"  "+st.Muted.Render(o[1]))
		}
		lines = append(lines, "", st.Muted.Render("enter or the number picks · esc cancels"))
		box = st.Popup(m.pick.title, lines, w, "")
	}
	if box != nil {
		body = ui.Overlay(body, box, m.w, max(0, (bh-len(box))/2))
	}
	rows = append(rows, body, m.footer())
	return strings.Join(rows, "\n")
}

// ------------------------------------------------------------- lists

func (m Model) table(cols []ui.Column, rows [][]string, w, h int) []string {
	sel := m.sel[m.sec]
	top := ui.Scroll(sel, m.top[m.sec], h-1)
	return m.st.Table(cols, rows, sel, top, w, h)
}

func (m Model) empty(text string, w int) []string {
	if m.filter.Value() != "" {
		text = "Nothing matches “" + m.filter.Value() + "”."
	}
	return ui.Wrap(m.st.Muted.Render(text), w)
}

func (m Model) list(w, h int) (string, []string) {
	st := m.st
	if !m.loaded {
		if m.err != "" {
			return "Docker", append([]string{st.Red.Render("Can't reach Docker")}, ui.Wrap(m.err+"\n\nIs the daemon running (systemctl start docker)? Are you in the docker group? nauka keeps trying.", w)...)
		}
		return "Docker", []string{st.Muted.Render("loading…")}
	}
	switch m.sec {
	case secContainers:
		cs := m.containers()
		running := 0
		for _, c := range cs {
			if c.Running() {
				running++
			}
		}
		title := fmt.Sprintf("Containers · %d running · %d stopped", running, len(cs)-running)
		if len(cs) == 0 {
			return title, m.empty("No containers. Images (Tab) can run one; Compose projects show up once started.", w)
		}
		var rows [][]string
		for _, c := range cs {
			dot := st.Muted.Render("○")
			short := shortName(c)
			name := st.Muted.Render(short)
			switch {
			case c.State == "running" && c.Health == "unhealthy":
				dot, name = st.Red.Render("●"), st.Bold.Render(short)
			case c.State == "running":
				dot, name = st.Green.Render("●"), st.Bold.Render(short)
			case c.State == "paused":
				dot, name = st.Yellow.Render("◐"), st.Bold.Render(short)
			case c.State == "restarting":
				dot = st.Peach.Render("●")
			}
			var ports []string
			for _, p := range c.Published {
				ports = append(ports, strconv.Itoa(p.HostPort))
			}
			cpu := ""
			if s, ok := m.stats[c.ID]; ok && c.State == "running" {
				cpu = fmt.Sprintf("%.1f%%", s.CPU)
			}
			status := st.Muted.Render(c.Status)
			if c.State == "running" {
				status = st.Muted.Render(dock.Ago(c.Started))
			}
			rows = append(rows, []string{dot, name, st.Sub.Render(c.Image), strings.Join(ports, " "), cpu, status})
		}
		return title, m.table([]ui.Column{{Title: "", Width: 1}, {Title: "name", Width: 0}, {Title: "image", Width: 20},
			{Title: "ports", Width: 11}, {Title: "cpu", Width: 6}, {Title: "up", Width: 8}}, rows, w, h)
	case secCompose:
		ps := m.projects()
		if len(ps) == 0 {
			return "Compose projects", m.empty("No compose projects running. docker compose up in a project folder makes one; it shows here with its services.", w)
		}
		var rows [][]string
		for _, p := range ps {
			dot := st.Muted.Render("○")
			if p.Running == p.All {
				dot = st.Green.Render("●")
			} else if p.Running > 0 {
				dot = st.Yellow.Render("◐")
			}
			rows = append(rows, []string{dot, st.Bold.Render(p.Name), fmt.Sprintf("%d of %d up", p.Running, p.All), st.Sub.Render(tilde(p.WorkDir))})
		}
		return fmt.Sprintf("Compose projects · %d", len(ps)), m.table([]ui.Column{{Title: "", Width: 1}, {Title: "project", Width: 18},
			{Title: "services", Width: 10}, {Title: "folder", Width: 0}}, rows, w, h)
	case secImages:
		is := m.images()
		var rows [][]string
		for _, im := range is {
			users := len(m.containersUsing("", im.ID))
			used := st.Muted.Render("—")
			if users > 0 {
				used = strconv.Itoa(users)
			}
			name := st.Text.Render(im.Repo) + st.Muted.Render(":"+shortTag(im.Tag))
			if im.Dangling {
				name, used = st.Yellow.Render("<none> "+dock.ShortID(im.ID)), st.Yellow.Render("dangling")
				if users > 0 {
					used = strconv.Itoa(users)
				}
			}
			rows = append(rows, []string{name, im.Size, st.Muted.Render(im.Created), used})
		}
		total := ""
		for _, u := range m.usage {
			if u.Type == "Images" {
				total = " · " + u.Size
			}
		}
		if len(is) == 0 {
			return "Images", m.empty("No images. docker pull, or a build, adds one.", w)
		}
		return fmt.Sprintf("Images · %d%s", len(is), total), m.table([]ui.Column{{Title: "image", Width: 0}, {Title: "size", Width: 9},
			{Title: "created", Width: 15}, {Title: "used by", Width: 8}}, rows, w, h)
	case secVolumes:
		vs := m.volumes()
		var rows [][]string
		for _, v := range vs {
			users := m.containersUsing(v.Name, "")
			used := st.Muted.Render("nothing")
			if len(users) == 1 {
				used = users[0].Name
			} else if len(users) > 1 {
				used = fmt.Sprintf("%d containers", len(users))
			}
			name := st.Text.Render(v.Name)
			if v.Anonymous {
				name = st.Muted.Render(shortNames([]string{v.Name})[0] + " (anonymous)")
				if len(users) == 0 {
					used = st.Yellow.Render("unused")
				}
			}
			rows = append(rows, []string{name, m.volSize[v.Name], used})
		}
		total := ""
		for _, u := range m.usage {
			if u.Type == "Local Volumes" {
				total = " · " + u.Size
			}
		}
		if len(vs) == 0 {
			return "Volumes", m.empty("No volumes. n creates one.", w)
		}
		return fmt.Sprintf("Volumes · %d%s", len(vs), total), m.table([]ui.Column{{Title: "name", Width: 0}, {Title: "size", Width: 9},
			{Title: "used by", Width: 24}}, rows, w, h)
	case secNetworks:
		ns := m.networks()
		var rows [][]string
		for _, n := range ns {
			name := st.Text.Render(n.Name)
			if n.Builtin() {
				name = st.Sub.Render(n.Name)
			}
			sub := n.Subnet
			if sub == "" {
				sub = st.Muted.Render("—")
			}
			rows = append(rows, []string{name, st.Muted.Render(n.Driver), sub, strconv.Itoa(len(n.Containers))})
		}
		return fmt.Sprintf("Networks · %d", len(ns)), m.table([]ui.Column{{Title: "name", Width: 0}, {Title: "driver", Width: 8},
			{Title: "subnet", Width: 16}, {Title: "containers", Width: 10}}, rows, w, h)
	case secSystem:
		var rows [][]string
		for _, u := range m.usage {
			rows = append(rows, []string{st.Bold.Render(u.Type), u.Total, u.Active, u.Size, st.Peach.Render(u.Reclaimable)})
		}
		if len(rows) == 0 {
			return "Disk use", []string{st.Muted.Render("asking docker…")}
		}
		lines := m.st.Table([]ui.Column{{Title: "kind", Width: 14}, {Title: "total", Width: 6}, {Title: "in use", Width: 7},
			{Title: "size", Width: 9}, {Title: "reclaimable", Width: 0}}, rows, -1, 0, w, h)
		lines = append(lines, "", st.Muted.Render("P picks what to prune; each asks first and says what goes."))
		return "Disk use (docker system df)", lines
	}
	return "", nil
}

// shortName drops the compose project from a container's name:
// "onadssuper-postgres-1" is "postgres-1" (the project shows beside it).
func shortName(c *dock.Container) string {
	if c.Project != "" {
		for _, sep := range []string{"-", "_"} {
			if rest, ok := strings.CutPrefix(c.Name, c.Project+sep); ok {
				return rest
			}
		}
	}
	return c.Name
}

func shortTag(t string) string {
	if len(t) > 14 {
		return t[:12] + "…"
	}
	return t
}

// ------------------------------------------------------------- details

func (m Model) details(w, h int) (string, []string) {
	if !m.loaded {
		return "", nil
	}
	switch m.sec {
	case secContainers:
		return m.containerDetails(w, h)
	case secCompose:
		return m.projectDetails(w, h)
	case secImages:
		return m.imageDetails(w, h)
	case secVolumes:
		return m.volumeDetails(w, h)
	case secNetworks:
		return m.networkDetails(w, h)
	case secSystem:
		return m.systemDetails(w)
	}
	return "", nil
}

// linkLine marks the i-th link of the details pane when it has focus.
func (m Model) linkLine(i int, text string) string {
	if m.focus == 1 && i == m.dsel {
		return m.st.Accent.Render("▌ ") + m.st.Highlight(text, m.dw-2)
	}
	return "  " + text
}

// scrolled shows lines from a scroll position so the selected one stays
// in view, marking it when the pane has focus.
func (m Model) scrolled(lines []string, h int, follow bool) []string {
	m.dsel = min(m.dsel, max(0, len(lines)-1))
	if len(lines) <= h {
		if m.focus == 1 {
			lines = append([]string(nil), lines...)
			if m.dsel < len(lines) {
				lines[m.dsel] = m.st.Highlight(lines[m.dsel], m.dw)
			}
		}
		return lines
	}
	top := 0
	if m.focus == 1 {
		top = max(0, min(m.dsel-h/2, len(lines)-h))
	} else if follow {
		top = len(lines) - h
	}
	out := append([]string(nil), lines[top:top+h]...)
	if m.focus == 1 && m.dsel >= top && m.dsel < top+h {
		out[m.dsel-top] = m.st.Highlight(out[m.dsel-top], m.dw)
	}
	return out
}

func (m Model) subtabs() string {
	st := m.st
	var b strings.Builder
	for i, n := range subNames {
		t := fmt.Sprintf("%d %s", i+1, n)
		if i == m.sub {
			b.WriteString(st.Chip(t, st.T.Text))
		} else {
			b.WriteString(st.Muted.Render(" " + t + " "))
		}
		b.WriteString(" ")
	}
	return b.String()
}

var secretKey = regexp.MustCompile(`(?i)pass|secret|token|key|auth|credential|private|dsn|database_url`)

func (m Model) containerDetails(w, h int) (string, []string) {
	st := m.st
	c := m.curContainer()
	if c == nil {
		return "", nil
	}
	out := []string{m.subtabs(), ""}
	room := h - 2
	switch m.sub {
	case subOverview:
		img := c.Image
		for _, im := range m.snap.Images {
			if im.ID == c.ImageID {
				img += st.Muted.Render(" · " + im.Size)
			}
		}
		status := c.Status
		switch {
		case c.State == "running" && c.Health == "unhealthy":
			status = st.Red.Render(status)
		case c.State == "running":
			status = st.Green.Render(status)
		case c.State == "exited":
			status = st.Muted.Render(status)
		}
		kv := [][2]string{{"status", status}}
		if c.Project != "" {
			kv = append(kv, [2]string{"compose", c.Project + " / " + c.Service + st.Muted.Render(" · "+tilde(c.WorkDir))})
		}
		var ports []string
		for _, p := range c.Published {
			ports = append(ports, fmt.Sprintf("%s:%s → %d/%s", p.HostIP, st.Bold.Render(strconv.Itoa(p.HostPort)), p.CtrPort, p.Proto))
		}
		kv = append(kv, [2]string{"ports", strings.Join(ports, ", ")}, [2]string{"restart", c.Restart}, [2]string{"command", st.Sub.Render(c.Cmd)})
		out = append(out, st.KV(kv, 9)...)
		links := m.links()
		li := 0
		out = append(out, "", st.Heading("image"))
		out = append(out, m.linkLine(li, st.Accent.Render(img)))
		li++
		var vols, binds []dock.Mount
		for _, mt := range c.Mounts {
			if mt.Type == "volume" {
				vols = append(vols, mt)
			} else {
				binds = append(binds, mt)
			}
		}
		out = append(out, "", st.Heading(fmt.Sprintf("volumes · %d", len(c.Mounts))))
		if len(c.Mounts) == 0 {
			out = append(out, "  "+st.Muted.Render("none"))
		}
		for _, mt := range vols {
			rw := "rw"
			if !mt.RW {
				rw = "ro"
			}
			size := m.volSize[mt.Name]
			if size != "" {
				size = " · " + size
			}
			icon := ""
			if st.Nerd {
				icon = "󰋊 "
			}
			out = append(out, m.linkLine(li, st.Accent.Render(icon+shortNames([]string{mt.Name})[0])+" → "+mt.Dest+st.Muted.Render(size+" · "+rw)))
			li++
		}
		for _, mt := range binds {
			rw := "rw"
			if !mt.RW {
				rw = "ro"
			}
			icon := ""
			if st.Nerd {
				icon = "󰉋 "
			}
			out = append(out, "  "+st.Sub.Render(icon+tilde(mt.Source))+" → "+mt.Dest+st.Muted.Render(" · "+mt.Type+" · "+rw))
		}
		out = append(out, "", st.Heading(fmt.Sprintf("networks · %d", len(c.Networks))))
		for _, n := range c.Networks {
			var with []string
			for _, nw := range m.snap.Networks {
				if nw.Name == n.Network {
					for _, mc := range nw.Containers {
						if mc.Name != c.Name {
							with = append(with, mc.Name)
						}
					}
				}
			}
			extra := ""
			if len(with) > 0 {
				extra = " · with " + strings.Join(with, ", ")
			}
			icon := ""
			if st.Nerd {
				icon = "󰛳 "
			}
			ip := n.IP
			if ip == "" {
				ip = "no address (stopped)"
			}
			out = append(out, m.linkLine(li, st.Accent.Render(icon+n.Network)+" "+ip+st.Muted.Render(extra)))
			li++
		}
		_ = links
		if s, ok := m.stats[c.ID]; ok && c.State == "running" {
			out = append(out, "", m.bars(s, w))
		}
		if m.focus == 1 {
			out = append(out, "", st.Muted.Render("enter jumps to the image, volume or network · backspace comes back"))
		} else {
			out = append(out, "", st.Muted.Render("enter: pick a volume or network to jump to it"))
		}
	case subLogs:
		lines := m.logs[c.ID]
		if lines == nil {
			out = append(out, st.Muted.Render("reading…"))
			break
		}
		var styled []string
		for _, l := range lines {
			styled = append(styled, logLine(st, l, w))
		}
		out = append(out, m.scrolled(styled, room, true)...)
	case subStats:
		s, ok := m.stats[c.ID]
		if !ok || c.State != "running" {
			out = append(out, st.Muted.Render("not running"))
			break
		}
		out = append(out, m.bars(s, w), "")
		out = append(out, st.KV([][2]string{{"memory", s.Mem + " of " + s.MemLimit}, {"network", s.NetIO + st.Muted.Render("  (in / out)")},
			{"disk", s.BlockIO + st.Muted.Render("  (read / written)")}, {"processes", s.PIDs}}, 10)...)
	case subEnv:
		var lines []string
		for _, e := range c.Env {
			k, v, _ := strings.Cut(e, "=")
			if secretKey.MatchString(k) && !m.reveal {
				v = st.Muted.Render("•••••••• (v shows it)")
			}
			lines = append(lines, st.Accent.Render(k)+"="+v)
		}
		out = append(out, m.scrolled(lines, room, false)...)
	case subInspect:
		out = append(out, m.scrolled(m.inspectLines(c), room, false)...)
	case subFiles:
		out[1] = st.Bold.Render(m.fileDir) + st.Muted.Render("  enter opens a folder · backspace goes up · c copies to ~/Downloads")
		if m.fileErr != "" {
			out = append(out, st.Muted.Render(m.fileErr))
			break
		}
		var lines []string
		for _, f := range m.files {
			name := f.Name
			switch {
			case f.Dir:
				name = st.Blue.Render(name + "/")
			case f.Link:
				name = st.Teal.Render(name + "@")
			}
			lines = append(lines, st.Muted.Render(ui.Fit(f.Mode, 11))+" "+ui.Fit(f.Size, 9)+" "+name)
		}
		if lines == nil {
			lines = []string{st.Muted.Render("reading…")}
		}
		out = append(out, m.scrolled(lines, room, false)...)
	}
	return c.Name, out
}

func (m Model) bars(s dock.Stats, w int) string {
	st := m.st
	bw := max(8, min(20, (w-40)/2))
	bar := func(pct float64, col string) string {
		n := int(pct / 100 * float64(bw))
		n = max(0, min(bw, n))
		if pct > 0 && n == 0 {
			n = 1
		}
		return ui.FG(col).Render(strings.Repeat("━", n)) + st.Muted.Render(strings.Repeat("─", bw-n))
	}
	cpuCol := st.T.Green
	if s.CPU > 80 {
		cpuCol = st.T.Red
	}
	return st.Muted.Render("cpu ") + bar(min(s.CPU, 100), cpuCol) + fmt.Sprintf(" %.1f%%", s.CPU) + "   " +
		st.Muted.Render("mem ") + bar(s.MemPerc, st.T.Blue) + " " + s.Mem
}

var reTS = regexp.MustCompile(`^\d{4}-\d\d-\d\dT(\d\d:\d\d:\d\d)\.\d+Z `)
var reErr = regexp.MustCompile(`(?i)\b(error|fatal|panic|exception|failed)\b`)
var reWarn = regexp.MustCompile(`(?i)\bwarn(ing)?\b`)

func logLine(st ui.Styles, l string, w int) string {
	ts := ""
	if m := reTS.FindStringSubmatch(l); m != nil {
		ts, l = st.Muted.Render(m[1]+" "), l[len(m[0]):]
	}
	switch {
	case reErr.MatchString(l):
		l = st.Red.Render(l)
	case reWarn.MatchString(l):
		l = st.Yellow.Render(l)
	}
	return ui.Fit(ts+l, w)
}

func (m Model) projectDetails(w, h int) (string, []string) {
	st := m.st
	p := m.curProject()
	if p == nil {
		return "", nil
	}
	var files []string
	for _, f := range p.ConfigFiles {
		files = append(files, tilde(f))
	}
	out := st.KV([][2]string{{"folder", tilde(p.WorkDir)}, {"file", strings.Join(files, ", ")},
		{"running", fmt.Sprintf("%d of %d services", p.Running, p.All)}}, 8)
	if m.showYAML {
		out = append(out, "", st.Heading(strings.Join(files, ", "))+st.Muted.Render("  y shows the services"))
		var lines []string
		for _, l := range m.compose[p.Name] {
			lines = append(lines, yamlLine(st, l))
		}
		return p.Name, append(out, m.scrolled(lines, h-len(out)-2, false)...)
	}
	out = append(out, "", st.Heading("services")+st.Muted.Render("  y shows the compose file"))
	i := 0
	for _, c := range m.snap.Containers {
		if c.Project != p.Name {
			continue
		}
		dot := st.Muted.Render("○")
		if c.State == "running" {
			dot = st.Green.Render("●")
		}
		out = append(out, m.linkLine(i, dot+" "+st.Bold.Render(ui.Fit(c.Service, 14))+" "+st.Sub.Render(ui.Fit(c.Image, 24))+" "+st.Muted.Render(c.Status)))
		i++
	}
	out = append(out, "", st.Muted.Render("u up · d down · r restart · b build · p pull · l logs · e edit the file"))
	return p.Name, out
}

func yamlLine(st ui.Styles, l string) string {
	trim := strings.TrimLeft(l, " ")
	indent := l[:len(l)-len(trim)]
	if strings.HasPrefix(trim, "#") {
		return indent + st.Muted.Render(trim)
	}
	if k, v, ok := strings.Cut(trim, ":"); ok && !strings.HasPrefix(trim, "- ") && !strings.Contains(k, " ") {
		col := st.Blue
		if indent == "" {
			col = st.Peach
		}
		return indent + col.Render(k) + ":" + v
	}
	return l
}

func (m Model) imageDetails(w, h int) (string, []string) {
	st := m.st
	im := m.curImage()
	if im == nil {
		return "", nil
	}
	d, ok := m.imgs[im.ID]
	kv := [][2]string{{"id", dock.ShortID(im.ID)}, {"size", im.Size}, {"created", im.Created}}
	if ok {
		kv = append(kv, [2]string{"layers", strconv.Itoa(len(d.Layers))}, [2]string{"platform", d.Platform},
			[2]string{"runs", st.Sub.Render(d.Entrypoint)}, [2]string{"exposes", strings.Join(d.Exposed, ", ")},
			[2]string{"volumes", strings.Join(d.Volumes, ", ")})
	}
	out := st.KV(kv, 9)
	if m.layers && ok {
		out = append(out, "", st.Heading("layers (docker history)")+st.Muted.Render("  H hides them"))
		var lines []string
		for _, l := range d.Layers {
			lines = append(lines, ui.Fit(l, w))
		}
		return im.Ref(), append(out, m.scrolled(lines, h-len(out)-2, false)...)
	}
	users := m.containersUsing("", im.ID)
	out = append(out, "", st.Heading(fmt.Sprintf("containers from it · %d", len(users))))
	if len(users) == 0 {
		out = append(out, "  "+st.Muted.Render("none"))
	}
	for i, c := range users {
		dot := st.Muted.Render("○")
		if c.State == "running" {
			dot = st.Green.Render("●")
		}
		out = append(out, m.linkLine(i, dot+" "+st.Accent.Render(c.Name)+" "+st.Muted.Render(c.Status)))
	}
	if ok && len(d.Tags) > 0 {
		out = append(out, "", st.Heading("tags"), "  "+strings.Join(d.Tags, " · "))
	}
	out = append(out, "", st.Muted.Render("r run · p pull newer · H layers · d remove · P prune dangling"))
	return im.Ref(), out
}

func (m Model) volumeDetails(w, h int) (string, []string) {
	st := m.st
	v := m.curVolume()
	if v == nil {
		return "", nil
	}
	size := m.volSize[v.Name]
	if size == "" {
		size = st.Muted.Render("asking docker…")
	}
	kv := [][2]string{{"driver", v.Driver}, {"size", size}, {"mount", st.Sub.Render(v.Mountpoint)}, {"created", strings.Split(v.Created, "T")[0]}}
	if p := v.Labels["com.docker.compose.project"]; p != "" {
		kv = append(kv, [2]string{"compose", p + " / " + v.Labels["com.docker.compose.volume"]})
	}
	out := st.KV(kv, 8)
	users := m.containersUsing(v.Name, "")
	out = append(out, "", st.Heading(fmt.Sprintf("containers using it · %d", len(users))))
	if len(users) == 0 {
		out = append(out, "  "+st.Muted.Render("none"))
	}
	for i, c := range users {
		dest, rw := "", "rw"
		for _, mt := range c.Mounts {
			if mt.Name == v.Name {
				dest = mt.Dest
				if !mt.RW {
					rw = "ro"
				}
			}
		}
		dot := st.Muted.Render("○")
		if c.State == "running" {
			dot = st.Green.Render("●")
		}
		out = append(out, m.linkLine(i, dot+" "+st.Accent.Render(c.Name)+" → "+dest+st.Muted.Render(" "+rw)))
	}
	out = append(out, "", st.Muted.Render("f browse its files · d remove · n new · P prune unused"))
	return shortNames([]string{v.Name})[0], out
}

func (m Model) networkDetails(w, h int) (string, []string) {
	st := m.st
	n := m.curNetwork()
	if n == nil {
		return "", nil
	}
	kv := [][2]string{{"driver", n.Driver}, {"scope", n.Scope}, {"subnet", n.Subnet}, {"gateway", n.Gateway}}
	if n.Internal {
		kv = append(kv, [2]string{"internal", "yes: no way out to the internet"})
	}
	if n.Project != "" {
		kv = append(kv, [2]string{"compose", n.Project})
	}
	out := st.KV(kv, 8)
	out = append(out, "", st.Heading(fmt.Sprintf("containers on it · %d", len(n.Containers))))
	if len(n.Containers) == 0 {
		out = append(out, "  "+st.Muted.Render("none"))
	}
	for i, c := range n.Containers {
		alias := ""
		if ct := m.containerByName(c.Name); ct != nil && ct.Service != "" {
			alias = " · " + ct.Service
		}
		out = append(out, m.linkLine(i, st.Accent.Render(c.Name)+" "+c.IP+st.Muted.Render(alias)))
	}
	out = append(out, "", st.Muted.Render("c connect a container · x disconnect one · n new · d remove · P prune"))
	return n.Name, out
}

func (m Model) systemDetails(w int) (string, []string) {
	st := m.st
	var stopped, dangling, unusedVol int
	for _, c := range m.snap.Containers {
		if !c.Running() {
			stopped++
		}
	}
	for _, im := range m.snap.Images {
		if im.Dangling && len(m.containersUsing("", im.ID)) == 0 {
			dangling++
		}
	}
	for _, v := range m.snap.Volumes {
		if len(m.containersUsing(v.Name, "")) == 0 {
			unusedVol++
		}
	}
	out := st.KV([][2]string{{"engine", "docker " + m.version}, {"containers", fmt.Sprintf("%d (%d stopped)", len(m.snap.Containers), stopped)},
		{"images", fmt.Sprintf("%d (%d dangling)", len(m.snap.Images), dangling)}, {"volumes", fmt.Sprintf("%d (%d unused)", len(m.snap.Volumes), unusedVol)},
		{"networks", strconv.Itoa(len(m.snap.Networks))}}, 11)
	out = append(out, "", st.Heading("prune"), "  "+st.Key("P", "pick what: stopped containers, images, volumes,"),
		"  "+st.Muted.Render("      networks, build cache, or everything unused"))
	return "Docker", out
}

// ------------------------------------------------------------- popups

func (m Model) helpBox(bh int) []string {
	st := m.st
	w := min(m.w-4, 100)
	var lines []string
	where := "-"
	for _, k := range Keys {
		if k.Where != where {
			where = k.Where
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, st.Heading(whereNames[where]))
		}
		lines = append(lines, st.Accent.Render(ui.Fit(k.Keys, 16))+" "+k.Desc)
	}
	room := max(3, bh-4)
	top := min(m.helpTop, max(0, len(lines)-room))
	lines = lines[top:min(len(lines), top+room)]
	return st.Popup("keys · j/k scroll · any key closes", lines, w, "")
}

func (m Model) footer() string {
	st := m.st
	mode := secKeys[m.sec]
	var keys [][2]string
	switch {
	case m.filtering:
		mode, keys = "search", [][2]string{{"↵", "keep"}, {"esc", "clear"}}
	case m.sec == secContainers:
		keys = [][2]string{{"s", "start/stop"}, {"r", "restart"}, {"p", "pause"}, {"e", "shell"}, {"l", "logs"}, {"1-6", "details"}, {"o", "open port"}, {"d", "remove"}, {"tab", "sections"}, {"?", "keys"}}
	case m.sec == secCompose:
		keys = [][2]string{{"u", "up"}, {"d", "down"}, {"r", "restart"}, {"b", "build"}, {"p", "pull"}, {"l", "logs"}, {"e", "edit"}, {"y", "file"}, {"?", "keys"}}
	case m.sec == secImages:
		keys = [][2]string{{"r", "run"}, {"p", "pull"}, {"H", "layers"}, {"d", "remove"}, {"P", "prune"}, {"?", "keys"}}
	case m.sec == secVolumes:
		keys = [][2]string{{"f", "browse"}, {"n", "new"}, {"d", "remove"}, {"P", "prune"}, {"?", "keys"}}
	case m.sec == secNetworks:
		keys = [][2]string{{"c", "connect"}, {"x", "disconnect"}, {"n", "new"}, {"d", "remove"}, {"P", "prune"}, {"?", "keys"}}
	case m.sec == secSystem:
		keys = [][2]string{{"P", "prune…"}, {"ctrl+r", "reload"}, {"?", "keys"}}
	}
	if m.focus == 1 && !m.filtering {
		keys = append([][2]string{{"j/k", "move"}, {"↵", "jump"}, {"⌫", "back"}, {"esc", "list"}}, keys...)
	}
	msg := ""
	if m.msg != "" {
		if m.msgErr {
			msg = st.Red.Render(m.msg)
		} else {
			msg = st.Text.Render(m.msg)
		}
	}
	return st.Footer(mode, keys, msg, m.w)
}
