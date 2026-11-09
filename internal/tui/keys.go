package tui

import (
	"github.com/Theyashsawarkar/vyakarana/keymap"
	"github.com/Theyashsawarkar/vyakarana/mode"
)

// Where a key works: the section the list shows.
const (
	inContainers = "containers"
	inCompose    = "compose"
	inImages     = "images"
	inVolumes    = "volumes"
	inNetworks   = "networks"
	inSystem     = "system"
)

var typing = []mode.Mode{mode.Insert}

// Keys is every key of the screen, in the order the help sheet lists
// them, with the vim grammar every Vayu tool shares (vyakarana): NORMAL
// keys, a Space leader, INSERT while searching. The README's key tables
// and scripts/keybindings (Vayu's keybinding search) come from it: after
// a change run `go test ./internal/tui -update`.
var Keys = keymap.New("nauka",
	keymap.Common{Noun: "row", Tabs: false,
		Search:  "Search the list (filters it)",
		Refresh: "Reload everything now (it also reloads every 2 s)"}.Binds(),
	[]keymap.Binding{
		{Seq: "L", Alt: []string{"Tab"}, Act: keymap.ActNextTab, Desc: "Switch to the next section (Containers, Compose, Images, Volumes, Networks, System)"},
		{Seq: "H", Alt: []string{"Shift+Tab"}, Act: keymap.ActPrevTab, Desc: "Switch to the previous section"},
		{Seq: "l", Act: "details", Desc: "Focus the details pane"},
		{Seq: "h", Act: "list", Desc: "Focus the list (leave the details pane)"},
		{Seq: "Enter", Act: "enter", Desc: "Focus the details pane; in it, jump to the volume, network or container under the cursor"},
		{Seq: "Backspace", Alt: []string{"Ctrl+o"}, Act: "back", Desc: "Go back to where Enter jumped from (or up a folder in Files)"},
		{Seq: "Ctrl+r", Act: keymap.ActRefresh, Desc: "Reload everything now", Hide: true},
		{Seq: "Ctrl+c", Act: keymap.ActQuit, Desc: "Quit", Hide: true},
		{Seq: "Space t c", Act: "sec-containers", Desc: "Switch to Containers"},
		{Seq: "Space t p", Act: "sec-compose", Desc: "Switch to Compose"},
		{Seq: "Space t i", Act: "sec-images", Desc: "Switch to Images"},
		{Seq: "Space t v", Act: "sec-volumes", Desc: "Switch to Volumes"},
		{Seq: "Space t n", Act: "sec-networks", Desc: "Switch to Networks"},
		{Seq: "Space t s", Act: "sec-system", Desc: "Switch to System"},
		{Seq: "y", Alt: []string{"Space y n"}, Act: "copy-name", Desc: "Copy the name of the selected row"},
		{Seq: "Space y i", Act: "copy-id", Desc: "Copy the ID of the selected row"},
		{Seq: "Space d P", Alt: []string{"P"}, Act: "prune-menu", Desc: "Remove what you pick: stopped containers, images, volumes, networks, build cache, everything (asks first)", Where: inSystem},

		// Containers
		{Seq: "1", Alt: []string{"Space t o"}, Act: "sub-1", Desc: "Switch the details to Overview", Where: inContainers},
		{Seq: "2", Alt: []string{"Space t l"}, Act: "sub-2", Desc: "Switch the details to Logs", Where: inContainers},
		{Seq: "3", Alt: []string{"Space t a"}, Act: "sub-3", Desc: "Switch the details to Stats", Where: inContainers},
		{Seq: "4", Alt: []string{"Space t e"}, Act: "sub-4", Desc: "Switch the details to Env", Where: inContainers},
		{Seq: "5", Alt: []string{"Space t j"}, Act: "sub-5", Desc: "Switch the details to Inspect (the JSON)", Where: inContainers},
		{Seq: "6", Alt: []string{"Space t f"}, Act: "sub-6", Desc: "Switch the details to Files", Where: inContainers},
		{Seq: "x", Alt: []string{"Space c s"}, Act: "start-stop", Desc: "Start or stop the container", Where: inContainers},
		{Seq: "Space c r", Act: "restart", Desc: "Restart the container", Where: inContainers},
		{Seq: "Space c p", Act: "pause", Desc: "Pause or unpause the container", Where: inContainers},
		{Seq: "Space c l", Act: "logs", Desc: "Follow the container's logs full screen (Ctrl+c comes back)", Where: inContainers},
		{Seq: "Space c x", Alt: []string{"Space o t"}, Act: "shell", Desc: "Open a shell inside the container (bash, else sh)", Where: inContainers},
		{Seq: "o", Alt: []string{"Space o b"}, Act: "open-port", Desc: "Open the container's published port in the browser", Where: inContainers},
		{Seq: "d d", Alt: []string{"Space d d"}, Act: "remove", Desc: "Remove the container (asks first)", Where: inContainers},
		{Seq: "Space u s", Act: "secrets", Desc: "Toggle showing secret values in Env", Where: inContainers},
		{Seq: "Space y f", Act: "copy-file", Desc: "Copy the selected file out of the container to ~/Downloads (Files)", Where: inContainers},
		{Seq: "s", Moved: "x", Where: inContainers},
		{Seq: "p", Moved: "Space c p", Where: inContainers},
		{Seq: "e", Moved: "Space c x", Where: inContainers},
		{Seq: "v", Moved: "Space u s", Where: inContainers},
		{Seq: "c", Moved: "Space y f", Where: inContainers},

		// Compose
		{Seq: "Space p u", Act: "up", Desc: "Start the compose project (docker compose up -d)", Where: inCompose},
		{Seq: "Space p d", Alt: []string{"d d"}, Act: "down", Desc: "Stop and remove the compose project's containers (docker compose down; asks first)", Where: inCompose},
		{Seq: "Space p r", Act: "restart", Desc: "Restart the compose project", Where: inCompose},
		{Seq: "Space p b", Alt: []string{"b"}, Act: "build", Desc: "Build the compose project's images", Where: inCompose},
		{Seq: "Space p p", Act: "pull", Desc: "Pull the compose project's images", Where: inCompose},
		{Seq: "Space p l", Act: "logs", Desc: "Follow the logs of every service of the project", Where: inCompose},
		{Seq: "Space e c", Act: "edit", Desc: "Edit the compose file in $EDITOR", Where: inCompose},
		{Seq: "Space u y", Act: "yaml", Desc: "Toggle the details between the services and the compose file", Where: inCompose},
		{Seq: "u", Moved: "Space p u", Where: inCompose},
		{Seq: "p", Moved: "Space p p", Where: inCompose},
		{Seq: "e", Moved: "Space e c", Where: inCompose},

		// Images
		{Seq: "Space i r", Act: "run", Desc: "Run a container from the image (asks for the options)", Where: inImages},
		{Seq: "Space i p", Act: "pull", Desc: "Pull a newer copy of the image", Where: inImages},
		{Seq: "Space u l", Act: "layers", Desc: "Toggle the image's layers (docker history)", Where: inImages},
		{Seq: "d d", Alt: []string{"Space d d"}, Act: "remove", Desc: "Remove the image (asks first)", Where: inImages},
		{Seq: "Space d p", Alt: []string{"P"}, Act: "prune", Desc: "Remove dangling images (docker image prune; asks first)", Where: inImages},
		{Seq: "p", Moved: "Space i p", Where: inImages},

		// Volumes
		{Seq: "Space o t", Act: "browse", Desc: "Open a shell in the volume's files (a throwaway alpine container)", Where: inVolumes},
		{Seq: "Space a v", Act: "create", Desc: "Create a volume", Where: inVolumes},
		{Seq: "d d", Alt: []string{"Space d d"}, Act: "remove", Desc: "Remove the volume (asks first; not while a container uses it)", Where: inVolumes},
		{Seq: "Space d p", Alt: []string{"P"}, Act: "prune", Desc: "Remove unused volumes (docker volume prune; asks first)", Where: inVolumes},
		{Seq: "f", Moved: "Space o t", Where: inVolumes},
		{Seq: "n", Moved: "Space a v", Where: inVolumes},

		// Networks
		{Seq: "Space a c", Act: "connect", Desc: "Connect a container to the network", Where: inNetworks},
		{Seq: "Space d c", Act: "disconnect", Desc: "Disconnect a container from the network", Where: inNetworks},
		{Seq: "Space a n", Act: "create", Desc: "Create a network", Where: inNetworks},
		{Seq: "d d", Alt: []string{"Space d d"}, Act: "remove", Desc: "Remove the network (asks first)", Where: inNetworks},
		{Seq: "Space d p", Alt: []string{"P"}, Act: "prune", Desc: "Remove unused networks (docker network prune; asks first)", Where: inNetworks},
		{Seq: "c", Moved: "Space a c", Where: inNetworks},
		{Seq: "x", Moved: "Space d c", Where: inNetworks},
		{Seq: "n", Moved: "Space a n", Where: inNetworks},

		// Searching
		{Seq: "Esc", Act: "insert-done", Desc: "Stop typing and keep the search (back to NORMAL)", Modes: typing},
		{Seq: "Enter", Act: "insert-done", Desc: "Stop typing and keep the search", Modes: typing, Hide: true},
		{Seq: "Ctrl+j", Alt: []string{"Down"}, Act: keymap.ActDown, Desc: "Move to the next row while typing", Modes: typing},
		{Seq: "Ctrl+k", Alt: []string{"Up"}, Act: keymap.ActUp, Desc: "Move to the previous row while typing", Modes: typing},
	}).
	Group("Space c", "Container").
	Group("Space p", "Compose project").
	Group("Space i", "Image").
	Where(inContainers, "Containers").
	Where(inCompose, "Compose").
	Where(inImages, "Images").
	Where(inVolumes, "Volumes").
	Where(inNetworks, "Networks").
	Where(inSystem, "System")
