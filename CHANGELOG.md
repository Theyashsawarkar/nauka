# Changelog

Newest first. Releases are tags (`vX.Y.Z`).

## Unreleased

- The website has Vayu's Spatial look: graphite glass, a floating navigation bar, near-white for links and the main button, and the tool's own colour only as its signature (the logo's glow, the shloka's line, the release dots). No more all-caps labels.

## v0.2.0 (2026-11-09)

- Vim keys, the grammar every Vayu tool now shares (from the new shared
  module vyakarana): NORMAL and INSERT modes (`/` searches, `Esc` goes back
  and keeps the search), counts (`5j`, `3gg`), `g g` / `G`, `H` / `L` for
  sections, `h` / `l` between the list and the details, `dd` to remove,
  `y` copies the name (`Space y i` the ID, new), `x` starts or stops a
  container, and a `Space` leader with a which-key box after 300 ms:
  `Space c` container, `Space p` compose project, `Space i` image,
  `Space a` create, `Space d` remove and prune, `Space u` toggles,
  `Space t` sections and details. `Space Space` searches every action; `?`
  no longer closes on a stray key. Deleting asks first and only `y` says yes.
- Moved keys say where they went (see the README's Keys). `r` reloads now
  (it restarted a container or ran an image), `l` focuses the details (it
  followed logs), `H` is the previous section (it showed an image's layers).
- The README's key tables and `scripts/keybindings` are generated from the
  keymap; the search shows each key's section and mode
  (`nauka containers:normal`).
- The drawing kit (`internal/ui`) now comes from vyakarana instead of a copy.
- Fixed: in a tmux popup, its key (`D`) now closes it from the help, a
  remove question (closing is a no) and a pick list, not only from the list.
  In the `Space Space` palette it's still a letter you type.
- Fixed: keys that arrive together (typed fast, or sent by tmux, like
  `gg` or `dd`) did nothing; they now work one by one.
- Fixed: answering no to a remove or prune left "removing …" on the
  status line for a few seconds, though nothing was removed.
- Every corner is rounded: the website's callouts.
- A logo and a shloka at the top of the README and the website: every Vayu tool has them now.

- Screenshots in the README and on the website.
- A website (GitHub Pages, https://theyashsawarkar.github.io/nauka/): the README as a home page, the changelog and architecture decision records (`docs/adr/`), built by `scripts/build-site.py` like the other Vayu tools' sites; CI checks it's up to date.

## v0.1.1 (2026-10-27)

- The built binary no longer holds a line Vayu's keybinding search took
  for a broken key description (it scans plugins' `bin/` folders).

## v0.1.0 (2026-10-27)

- First release: containers, compose projects, images, volumes, networks
  and disk use in one screen. A container's details show the volumes and
  networks it's attached to (Enter jumps to them, Backspace comes back),
  its logs, stats, environment (secrets hidden), inspect and files.
- Actions: start/stop, restart, pause, shell, follow logs, remove; compose
  up/down/restart/build/pull/logs/edit; image run/pull/remove; volume
  browse/create/remove; network connect/disconnect/create/remove; prune
  any kind, or everything. Deleting asks first and says what goes.
- tmux plugin: `prefix D` opens it in a popup.
