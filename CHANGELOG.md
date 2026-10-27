# Changelog

Newest first. Releases are tags (`vX.Y.Z`).

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
