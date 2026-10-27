// nauka: Docker in the terminal: containers, compose projects, images,
// volumes, networks and disk use.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theyashsawarkar/nauka/internal/tui"
)

// Version is nauka's version.
const Version = "0.1.1"

const usage = `nauka: Docker in the terminal.

  nauka                  the screen (? in it lists every key)
  nauka --popup KEY      run inside a tmux popup: KEY closes it too
  nauka --version

Keys, the tmux plugin and its options: https://github.com/Theyashsawarkar/nauka
`

func main() {
	fs := flag.NewFlagSet("nauka", flag.ExitOnError)
	popup := fs.String("popup", "", "")
	version := fs.Bool("version", false, "")
	fs.Usage = func() { fmt.Print(usage) }
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	_ = fs.Parse(args)
	switch {
	case *version:
		fmt.Println("nauka", Version)
		return
	case cmd == "help":
		fmt.Print(usage)
		return
	case cmd == "keybindings": // scripts/keybindings, for Vayu's keybinding search
		fmt.Print(tui.KeybindingsFile())
		return
	case cmd != "":
		fmt.Fprintf(os.Stderr, "nauka: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	p := tea.NewProgram(tui.New(tui.Options{PopupKey: *popup}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nauka:", err)
		os.Exit(1)
	}
}
