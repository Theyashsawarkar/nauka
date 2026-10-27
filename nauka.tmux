#!/usr/bin/env bash
# nauka as a tmux plugin: TPM entry point (also works with a plain
# `run-shell` on this file). Binds prefix + @nauka-key to open nauka (Docker:
# containers, compose, images, volumes, networks) in a popup; the same key
# inside the popup closes it.
#
# Options (set them before TPM's `run` line):
#   @nauka-key       key after the prefix (default D; "none" binds nothing)
#   @nauka-width     popup width  (default 90%)
#   @nauka-height    popup height (default 88%)
#   @nauka-title     popup title  (default " nauka ")
#   @nauka-launcher  your own popup command instead of display-popup: it is
#                    run with display-popup's arguments appended (-E -d DIR
#                    -T TITLE CMD), tmux formats expanded, e.g.
#                    "~/.tmux/scripts/popup.sh '#{client_name}'"
#
# To open it from a status-line click, open a popup running
# "<plugin dir>/scripts/nauka-run".

dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
run="$dir/scripts/nauka-run"

opt() { local v; v=$(tmux show-option -gqv "$1"); printf '%s' "${v:-$2}"; }
key=$(opt @nauka-key D)
[ "$key" = none ] && exit 0
width=$(opt @nauka-width 90%)
height=$(opt @nauka-height 88%)
title=$(opt @nauka-title ' nauka ')
launcher=$(opt @nauka-launcher '')

cmd="'$run' --popup '$key'"
if [ -n "$launcher" ]; then
  open="run -b \"$launcher -E -d '#{pane_current_path}' -T '$title' $cmd\""
else
  open="display-popup -E -w '$width' -h '$height' -d '#{pane_current_path}' -T '$title' \"$cmd\""
fi

tmux bind-key -N "Open nauka Docker popup (containers, compose, images, volumes, networks; prefix $key again closes)" "$key" "$open"
