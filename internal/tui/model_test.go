package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theyashsawarkar/nauka/internal/dock"
)

func demoModel() Model {
	m := New(Options{})
	m.w, m.h = 120, 40
	m.snap = dock.Snapshot{Containers: []dock.Container{
		{ID: "a1", Name: "web", State: "running"},
		{ID: "b2", Name: "db", State: "exited"},
		{ID: "c3", Name: "cache", State: "exited"},
	}}
	return m
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

// Saying no to a remove leaves no "removing …" behind: the docker
// command's status shows only once it runs.
func TestConfirmNoSaysNothing(t *testing.T) {
	for _, no := range []string{"n", "esc", "enter", "Y"} {
		m := press(demoModel(), "d", "d")
		if m.pop != popConfirm {
			t.Fatalf("d d: no confirm")
		}
		if m.msg != "" {
			t.Fatalf("d d: the status says %q before the answer", m.msg)
		}
		mm, cmd := m.Update(map[string]tea.KeyMsg{"n": {Type: tea.KeyRunes, Runes: []rune("n")},
			"esc": {Type: tea.KeyEsc}, "enter": {Type: tea.KeyEnter}, "Y": {Type: tea.KeyRunes, Runes: []rune("Y")}}[no])
		m = mm.(Model)
		if cmd != nil || m.pop != popNone || m.msg != "" {
			t.Errorf("%s: cmd %v, pop %d, status %q; want nothing done", no, cmd != nil, m.pop, m.msg)
		}
	}
}

// Keys that arrive together in one message ("gg", typed fast or sent by
// tmux) work as if typed one by one.
func TestBurstOfKeys(t *testing.T) {
	m := press(demoModel(), "G")
	if m.sel[secContainers] != 2 {
		t.Fatalf("G: row %d", m.sel[secContainers])
	}
	if m = press(m, "gg"); m.sel[secContainers] != 0 {
		t.Errorf("gg in one message: row %d, want 0", m.sel[secContainers])
	}
	if m = press(m, "dd"); m.pop != popConfirm {
		t.Errorf("dd in one message: no confirm")
	}
}

// In a tmux popup, its key (D) closes it from the help, a remove question
// (closing is a "no": nothing runs) and a pick list, as it does from the list.
func TestPopupKeyClosesFromHelpAndQuestions(t *testing.T) {
	quits := func(m Model, where string) {
		t.Helper()
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		if cmd == nil {
			t.Fatalf("D in the %s: no command, want quit", where)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("D in the %s didn't quit", where)
		}
	}
	m := demoModel()
	m.opt.PopupKey = "D"
	if h := press(m, "?"); h.pop != popHelp {
		t.Fatalf("?: pop %d", h.pop)
	} else {
		quits(h, "help")
	}
	if c := press(m, "d", "d"); c.pop != popConfirm {
		t.Fatalf("d d: pop %d", c.pop)
	} else {
		quits(c, "question")
	}
	// The palette is typed into: D is a letter there.
	p := press(m, " ", " ")
	if p.pop != popPalette {
		t.Fatalf("Space Space: pop %d", p.pop)
	}
	if mm, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}); mm.(Model).pop != popPalette {
		t.Fatal("D in the palette closed it; it should type")
	}
}
