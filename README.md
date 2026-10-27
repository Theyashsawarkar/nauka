# nauka

**Docker in the terminal: containers, compose projects, images, volumes,
networks and disk use, with what's attached to what.** Start, stop, open a
shell, follow logs, bring a compose project up or down, prune, all from the
keyboard.

nauka (नौका) is Sanskrit for a boat.

```
 Containers 4   Compose 1   Images 15   Volumes 6   Networks 4   System     nauka · docker 29.9.1 · 3 running
╭─ Containers · 3 running · 1 stopped ───────────╮╭─ onadssuper-postgres-1 ─────────────────────────╮
│    NAME          IMAGE              PORTS  CPU │ │ 1 Overview  2 Logs  3 Stats  4 Env  5 Inspect  │
│ ▌● postgres-1    pgvector/pgvector  54320  0.5%│ │ status   up 25h · healthy                      │
│  ● redis-1       redis:8.2-alpine   63790  0.5%│ │ compose  onadssuper / postgres                  │
│  ● mailpit-1     axllent/mailpit    8025   0.0%│ │ VOLUMES · 2                                    │
│  ○ aura-local    redis:7-alpine           exited│ │  󰋊 onadssuper_postgres-data → /var/lib/...     │
╰────────────────────────────────────────────────╯│ NETWORKS · 1                                   │
                                                  │  󰛳 onadssuper_default 172.18.0.4 · with redis-1 │
 containers  s start/stop  r restart  p pause  e shell  l logs  1-6 details  d remove  ? keys
```

## Sections

Tab / Shift+Tab move between them. Each is a list on the left and the
selected item's details on the right; Enter focuses the details, where
Enter on a linked volume, network, image or container **jumps to it** in
its own section, and Backspace comes back.

- **Containers**: state, image, published ports, CPU, uptime. The details
  have six tabs (`1`–`6`):
  - **Overview**: status and health, compose project and service, ports,
    restart policy, command, the **image**, the **volumes** mounted (where,
    size, read-only or not, bind mounts too) and the **networks** it's on
    (its IP, and who else is there), with live CPU and memory.
  - **Logs**: the last 500 lines, refreshed every 2 s, errors in red and
    warnings in yellow. `l` follows them full screen.
  - **Stats**: CPU, memory, network and disk I/O, processes.
  - **Env**: the environment; values of secret-looking keys (password,
    token, key...) stay hidden until `v`.
  - **Inspect**: `docker inspect`, scrollable.
  - **Files**: browse the container's folders; `c` copies a file out to
    `~/Downloads`.
- **Compose**: every compose project (from its containers' labels), its
  services, folder and file. `u` up, `d` down, `r` restart, `b` build,
  `p` pull, `l` logs of every service, `e` edit the file, `y` show it.
- **Images**: size, age, which containers use each; details show the
  entrypoint, exposed ports, declared volumes, tags, and `H` the layers.
  `r` runs a container (asks for the `docker run` options), `p` pulls a
  newer copy.
- **Volumes**: size and the containers using each, anonymous ones marked.
  `f` opens a shell in its files (in a throwaway alpine container), `n`
  creates one.
- **Networks**: driver, subnet, gateway and who's on it with their IPs.
  `c` connects a container, `x` disconnects one, `n` creates one.
- **System**: `docker system df` (what takes how much, what's
  reclaimable) and `P` to prune: stopped containers, dangling or unused
  images, unused volumes or networks, the build cache, or everything.

### Safe by default

Anything that deletes asks first and says what goes (a volume's size, the
containers a down removes, how much a prune frees). A volume or image a
container still uses isn't removed: nauka says which containers to remove
first. Secrets in Env are hidden until you ask.

## Keys

| Key | Does |
|---|---|
| `j` / `k`, `Home` / `End`, `Ctrl+d` / `Ctrl+u` | Move |
| `Tab` / `Shift+Tab` | Switch section |
| `Enter` | Focus the details; on a link, jump to it |
| `Backspace` | Go back where you jumped from (up a folder in Files) |
| `Esc` | Leave the details, clear the search, or quit |
| `/` | Search the list |
| `Ctrl+r` | Reload now (it reloads every 2 s) |
| `?` | Show every key |
| `q` | Quit |

Containers: `s` start/stop · `r` restart · `p` pause/unpause · `e` shell
(bash, else sh) · `l` follow logs · `o` open its published port in the
browser · `d` remove · `1`–`6` details · `v` show secrets in Env · `c`
copy a file out (Files).

Compose: `u` up · `d` down · `r` restart · `b` build · `p` pull · `l`
logs · `e` edit · `y` compose file.

Images: `r` run · `p` pull · `H` layers · `d` remove · `P` prune dangling.

Volumes: `f` browse · `n` new · `d` remove · `P` prune unused.

Networks: `c` connect · `x` disconnect · `n` new · `d` remove · `P` prune.

System: `P` prune (pick what).

## Install

Needs Linux and the `docker` command (with the compose plugin for the
Compose section), and access to the daemon (the docker group, or rootless
Docker). nauka runs the docker command for everything, so it follows your
docker context exactly.

```sh
curl -fsSL https://raw.githubusercontent.com/Theyashsawarkar/nauka/main/install.sh | bash
```

downloads the release binary (or builds it with Go) into
`~/.local/share/nauka` and links `~/.local/bin/nauka`. Or `make install`
(`PREFIX`, `DESTDIR`), or `go install github.com/Theyashsawarkar/nauka/cmd/nauka@latest`.

### As a tmux plugin

```tmux
set -g @plugin 'Theyashsawarkar/nauka'
```

`prefix D` opens nauka in a popup; `D` inside closes it. Without nauka on
your `PATH`, the plugin builds it with Go the first time.

| Option | Default | |
|---|---|---|
| `@nauka-key` | `D` | the key after the prefix; `none` binds nothing |
| `@nauka-width` / `@nauka-height` | `90%` / `88%` | popup size |
| `@nauka-title` | ` nauka ` | popup title |
| `@nauka-launcher` | | your own popup command instead of `display-popup`; it gets display-popup's arguments (`-E -d DIR -T TITLE CMD`) |

To open it from a status-line click, open a popup that runs
`~/.tmux/plugins/nauka/scripts/nauka-run`.

## Colours

nauka follows [varna](https://github.com/Theyashsawarkar/varna)'s desktop
theme (`~/.local/state/varna/palette.json`, a `tools.nauka` pick if the
theme has one) and changes live with it; without varna it is Catppuccin
Mocha. Icons need a Nerd Font; `NO_NERD_FONT=1` turns them off.

## License

MIT
