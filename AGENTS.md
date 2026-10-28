# nauka -- notes for AI agents (same as CLAUDE.md)

Docker in the terminal: one Go binary (`cmd/nauka`) with a Bubble Tea
screen over the `docker` command (never the Docker API directly, so it
follows the user's context and permissions). Usable as the `nauka` command
(install.sh, make install) and as a TPM plugin (`nauka.tmux` binds a popup
key). It is pulled into Vayu (github.com/Theyashsawarkar/vayu) as a tmux
plugin, but must work on any Linux setup without Vayu: no hardcoded paths,
Vayu-specific behaviour only through the `@nauka-*` options.

- Layout: `internal/dock` runs docker and parses its JSON; `internal/tui`
  the screen (`model.go` state, keys and actions, `views.go` drawing);
  `internal/ui` the drawing kit shared in look with sandesh and pattan.
- Keys live in `internal/tui/keys.go`. After adding, changing or removing
  one: `go test ./internal/tui -update` (rewrites `scripts/keybindings`,
  which Vayu's keybinding search reads), and update the README's keys.
  Descriptions start with a verb the search knows.
- Anything that deletes asks first (`m.ask`) and says what is lost; a
  volume or image still in use is never removed. Env values of
  secret-looking keys stay hidden until `v`.
- The README documents every feature, key, option and command: a change to
  any of them updates it. Every change gets a CHANGELOG.md entry; releases
  are tags (`vX.Y.Z`) with `Version` in cmd/nauka/main.go bumped to match.
- `make check` must pass (vet, gofmt, ShellCheck, tests).
- Test on a throwaway tmux server, never the user's live one:
  `tmux -L t -f /dev/null new -d -s o -x 150 -y 40 "env -u TMUX bin/nauka"`,
  then `tmux -L t send-keys -t o ...` / `capture-pane -p -t o`, and
  `tmux -L t kill-server` after. Never stop, remove or prune the user's
  containers, images, volumes or networks in a test: only read.
- The website (docs/, GitHub Pages) is generated: after changing README.md, CHANGELOG.md or docs/adr/, run `python3 scripts/build-site.py` and commit docs/ (CI checks it). A significant design decision gets a new ADR in docs/adr/ (next number; supersede, never rewrite).
