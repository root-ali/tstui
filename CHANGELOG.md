# Changelog

All notable changes to tstui. Versions follow [Semantic Versioning](https://semver.org/).

## [0.3.0] - 2026-11-21

### Added

- Terminal: double-click selects a word and triple-click selects a line. Both copy to the clipboard, like a drag-selection. Paths, `user@host`, hostnames and IP addresses are selected whole. In programs that read the mouse (vim, htop), hold Shift.

## [0.2.0] - 2026-11-21

### Changed

- `/` opens an interactive search, like `kubens` or `fzf`. The server list narrows as you type, with no Enter needed. The footer shows the match count.
  - `↑` `↓`, `ctrl-p` `ctrl-n` and `pgup` `pgdn` move the cursor while you type.
  - `enter` connects to the highlighted server (`shift-enter` opens a new session), and `tab` keeps the filter and goes back to the list.
  - `esc` puts back the previous filter, and `ctrl-u` clears the text.
  - The bar starts with the current filter and uses the same filter language. Until a half-typed filter parses (`=web`), the last good one stays shown.
- `:filter expr` still sets a filter in one step. Before this release, `/` opened the palette prefilled with `:filter`.

## [0.1.0] - 2026-11-19

The first release: a k9s-style terminal UI for Teleport SSH, built on `tsh`.

### Highlights

- **Login popup.** Logs in with a password, OTP code, security key (WebAuthn, including key PIN) or SSO. Anything missing from the flags, environment or config is asked for in the popup. A valid `~/.tsh` profile is used as is.
- **Proxies and clusters.** Proxies come from the config file and from the `~/.tsh` profiles. Switch with `p`, a shortcut letter or `:proxy`. Root and leaf clusters are listed k9s-namespace style, with `0` showing every cluster.
- **Embedded SSH terminal.** Full TTY with 256 colours and truecolor, wide characters, mouse, bracketed paste and scrollback. Sessions open in tabs with tmux-style split panes, zoom and fullscreen. A `ctrl-b` prefix (configurable) gets you out of the terminal.
- **File transfer, port forwards and multi-exec.** `tsh scp` with path completion, a port forward manager with saved forwards, and one command on many servers with confirmation, risky-command detection and an audit log.

### Added

- Server list: a filter language (`web`, `!web`, `env=prod`, `env!=prod`, `env=`), favorites pinned at the top, sorting by name, address or cluster, and a reachability dot from the node heartbeat.
- Offline mode: the last good server list per proxy is cached and shown marked **STALE** when the proxy cannot be reached.
- Header: proxy, user and roles, tstui, tsh and server versions, login state with a certificate countdown (yellow under 30 minutes, red under 5), MFA type, latency, session count, filter, and the clusters panel.
- Health: proxies are pinged once a minute, and a warning appears when tsh is too old or too new for the server (one major version behind is supported). Expiry notices appear at 30 minutes, 5 minutes and expiry, with `L` to log in again. Open sessions are never closed because a certificate expired.
- Sessions: a login picker that remembers the choice per server, friendly errors (access denied, unreachable, MFA required, certificate expired), a disconnect banner with reconnect, optional auto-reconnect, scroll mode, and mouse selection copied with OSC 52.
- Command palette (`:`) with completion and history: `:proxy`, `:cluster`, `:login`, `:logout`, `:ssh`, `:sessions`, `:split`, `:vsplit`, `:fullscreen`, `:zoom`, `:filter`, `:sort`, `:fav`, `:refresh`, `:skin`, `:scp`, `:pf`, `:exec`, `:help`, `:quit`.
- Skins: `default` (k9s colours), `light` and `high-contrast`, plus skin files in `~/.config/tstui/skins/`. Three examples come in `examples/skins/`.
- Help overlay (`?`) and footer hints that follow the focused pane.
- CLI: `--proxy`, `--user`, `--password-stdin`, `--password-file`, `--password` (with a warning), `--mfa-token`, `--auth`, `--insecure`, `--config`, `--skin` and `--version`, plus the environment variables `TSTUI_PROXY`, `TSTUI_USER`, `TSTUI_PASSWORD` and `TSTUI_MFA`. Debugging: `--report`, `--log-file`, `--login-only`, `--demo-shell`.
- Robustness: a clear message when tsh is missing, clean shutdown on SIGINT, SIGTERM and SIGHUP (child processes are killed, the terminal restored), a crash handler that restores the terminal and never prints secrets, core dumps disabled, and no panics on tiny terminals.
- Security: passwords and codes are held as wipeable bytes, typed into tsh's prompt only (never in argv or the environment), and redacted from logs. Every file tstui writes is mode 0600. See `docs/security.md`.
- Packaging: static binaries (`CGO_ENABLED=0`) for linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64, as plain binaries and `.tar.gz`, `.deb` packages for Linux, `SHA256SUMS`, and a man page. The build works fully offline from vendored modules.
- Licensed under Apache-2.0. Archives and the `.deb` carry `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES`.
- Documentation: README, `docs/features.md`, `docs/architecture.md`, `docs/security.md`, `docs/verification-checklist.md`, CONTRIBUTING, and an example `config.yaml` with every key.

### Known limitations

**Tested against one tsh version.** tstui was verified with tsh v18.11.3 against Teleport v18.10.0 (community edition, local users, OTP). tsh v16 and v17 should work but have not been run, and v19 and newer show an "untested" warning. Several behaviours come from Teleport's source and docs and have not been seen on a real cluster yet. `docs/tsh-notes.md` lists them, and `docs/verification-checklist.md` is the plan to check them:

- the WebAuthn, key PIN and SSO login flows, and the exact line endings of their prompts
- the server's error texts for a wrong OTP, a locked user and an unknown auth connector. A wrong OTP may be reported as "wrong credentials". Patterns can be added with `tsh_prompts`.
- `tsh ssh` error texts (access denied, unreachable node), connecting to a hostname that two nodes share, and connecting to leaf-cluster nodes
- whether the proxy closes open sessions when the certificate expires (`disconnect_expired_cert`)

**Features**

- **Multi-exec** runs without a terminal, so nodes that need per-session MFA will probably fail there. Output is kept up to 256 KiB per stream per host.
- **Port forwards**: per-session MFA works with a key touch, but an OTP cannot be typed into a forward. Bind addresses are IPv4 or `localhost` only. The four-part `bind:port:host:port` form is unverified with tsh.
- **File transfer** shows a spinner and the elapsed time, not a progress bar. Local paths containing `:` are refused, and whether tsh glob-expands remote paths is unknown.
- **Terminal**: no kitty keyboard protocol, `modifyOtherKeys` or inline graphics (Sixel, kitty), and lines are not reflowed on resize. tmux and Midnight Commander were checked through their building blocks only, not run end to end. Inside a remote tmux, the `ctrl-b` prefix must be pressed twice, or set `keybindings.focus` to another key.
- **Config**: saving (favorites, saved forwards, `:skin`) rewrites `config.yaml` and drops its comments. The previous file is kept as `config.yaml.bak`.
- **Favorites** are stored by hostname, so a favorite applies in every cluster of the proxy. For a proxy that is only in `~/.tsh`, not in the config, favorites last for one run.
- **Reachability** comes from the node heartbeat in `tsh ls`. tstui never probes nodes, so the dot can say `●` for a node that has just gone down.
- A proxy address without a port is assumed to use 443.
- The password typed into the popup passes through a Go string inside tview, which cannot be wiped. Secrets from flags, files, stdin and the environment are wiped.

**Platforms and project**

- Linux and macOS only. No Windows: the login, the embedded terminal and port forwards need a Unix pseudo-terminal. A Windows build compiles, but it could only list servers, transfer files and run multi-exec, so it is not released.
- The race detector tests need cgo (`make race`). Release builds do not use cgo.
- The `.deb` maintainer field is a placeholder (`tstui maintainers`). The release workflow has not run yet, so the first tag is its first real test.

[0.3.0]: https://github.com/root-ali/tstui/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/root-ali/tstui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/root-ali/tstui/releases/tag/v0.1.0
