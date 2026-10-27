package tui

import (
	"fmt"
	"strings"
)

// Key is one key of the screen: what you press, what it does, and in
// which section (empty: everywhere).
type Key struct {
	Keys, Desc, Where string
}

// Keys is every key, in the order the ? sheet lists them. The README's key
// tables and scripts/keybindings (Vayu's keybinding search) come from it:
// after a change run `go test ./internal/tui -update`.
var Keys = []Key{
	{"j / k", "Move to the next / previous row", ""},
	{"Home / End", "Jump to the first / last row", ""},
	{"Tab / Shift+Tab", "Switch to the next / previous section (Containers, Compose, Images, Volumes, Networks, System)", ""},
	{"Enter", "Focus the details pane (Enter on a volume, network or container in it jumps there)", ""},
	{"Backspace", "Go back to where Enter jumped from (or up a folder in Files)", ""},
	{"Esc", "Leave the details pane, clear the search, or quit", ""},
	{"/", "Search the list (filters it)", ""},
	{"Ctrl+r", "Reload everything now (it also reloads every 2 s)", ""},
	{"?", "Show every key", ""},
	{"q", "Quit", ""},
	{"1 – 6", "Switch the container's details: Overview, Logs, Stats, Env, Inspect, Files", "containers"},
	{"s", "Start or stop the container", "containers"},
	{"r", "Restart the container", "containers"},
	{"p", "Pause or unpause the container", "containers"},
	{"e", "Open a shell inside the container (bash, else sh)", "containers"},
	{"l", "Follow the container's logs full screen (Ctrl+c comes back)", "containers"},
	{"o", "Open the container's published port in the browser", "containers"},
	{"d", "Remove the container (asks first)", "containers"},
	{"v", "Toggle showing secret values in Env", "containers"},
	{"c", "Copy the selected file out of the container to ~/Downloads (Files)", "containers"},
	{"u", "Start the compose project (docker compose up -d)", "compose"},
	{"d", "Stop and remove the compose project's containers (docker compose down; asks first)", "compose"},
	{"r", "Restart the compose project", "compose"},
	{"b", "Build the compose project's images", "compose"},
	{"p", "Pull the compose project's images", "compose"},
	{"l", "Follow the logs of every service of the project", "compose"},
	{"e", "Edit the compose file in $EDITOR", "compose"},
	{"y", "Toggle the details between the services and the compose file", "compose"},
	{"r", "Run a container from the image (asks for the options)", "images"},
	{"p", "Pull a newer copy of the image", "images"},
	{"H", "Toggle the image's layers (docker history)", "images"},
	{"d", "Remove the image (asks first)", "images"},
	{"P", "Remove dangling images (docker image prune; asks first)", "images"},
	{"f", "Open a shell in the volume's files (a throwaway alpine container)", "volumes"},
	{"n", "Create a volume", "volumes"},
	{"d", "Remove the volume (asks first; not while a container uses it)", "volumes"},
	{"P", "Remove unused volumes (docker volume prune; asks first)", "volumes"},
	{"c", "Connect a container to the network", "networks"},
	{"x", "Disconnect a container from the network", "networks"},
	{"n", "Create a network", "networks"},
	{"d", "Remove the network (asks first)", "networks"},
	{"P", "Remove unused networks (docker network prune; asks first)", "networks"},
	{"P", "Pick what to prune: stopped containers, images, volumes, networks, build cache, everything", "system"},
}

var whereNames = map[string]string{"": "everywhere", "containers": "Containers", "compose": "Compose", "images": "Images",
	"volumes": "Volumes", "networks": "Networks", "system": "System"}

// KeybindingsFile is scripts/keybindings, for Vayu's keybinding search.
func KeybindingsFile() string {
	var b strings.Builder
	b.WriteString("# Keys inside the nauka screen, for Vayu's keybinding search\n")
	b.WriteString("# (keybind-search.py reads ~/.tmux/plugins/*/scripts). Generated from\n")
	b.WriteString("# internal/tui/keys.go by `go test ./internal/tui -update`; don't edit.\n")
	for _, k := range Keys {
		where := ""
		if k.Where != "" {
			where = " (" + whereNames[k.Where] + ")"
		}
		fmt.Fprintf(&b, "# keybind: Tmux/nauka | %s | %s%s\n", k.Keys, k.Desc, where)
	}
	return b.String()
}
