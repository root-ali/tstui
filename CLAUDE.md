# tstui — Teleport SSH TUI

A k9s-style terminal UI for Teleport SSH, written in Go with tview and tcell. It wraps the `tsh` binary. Binary: `tstui`.

**Before touching anything that calls tsh, read `docs/tsh-notes.md`.** It records which tsh commands, JSON shapes and prompts are verified against a real tsh and which are still assumptions. When you verify an assumption, update it there.

## Wireframe

```
┌─ tstui ───────────────────────────────────────────────────────────────────────────────────┐
│ Project : tstui v0.1.0       User   : ali  (roles: access,editor)    Clusters            │
│ Proxy   : tp.ast.example ●   Login  : ✔ logged in · cert 07h42m      <0> all 155         │
│ Server  : v16.4.2            MFA    : webauthn/otp                   <1> teleport-ast 59 │
│ Client  : v16.4.2 (tsh)      Latency: 23ms   Sessions: 2             <2> afr 55          │
│ Cluster : teleport-ast       Filter : env=prod                       <3> hwb 41          │
├────────────────────────┬──────────────────────────────────────────────────────────────────┤
│ Servers (afr) [12/42]  │ [1:web-01 ●] [2:db-03 ●] [3:+]                                   │
│ ──────────────────────│ ┌──────────────────────────────────────────────────────────────┐ │
│ ★ web-01   prod  ●     │ │ ali@web-01:~$ _                                              │ │
│ ★ web-02   prod  ●     │ │            (embedded SSH terminal, full TTY)                 │ │
│   db-03    prod  ●     │ │                                                              │ │
│   cache-01 stage ○     │ └──────────────────────────────────────────────────────────────┘ │
├────────────────────────┴──────────────────────────────────────────────────────────────────┤
│ <enter>connect </>filter <f>fav <p>proxies <:>cmd <ctrl-b>focus <alt-1..9>tab <?>help <q> │
└───────────────────────────────────────────────────────────────────────────────────────────┘
```

- **Login popup** (user, masked password, MFA code, "touch your security key" state, Login/Cancel) opens at start when credentials are missing, and when the certificate expires.
- **Clusters panel** (header, right): the current proxy's root and leaf clusters from `tsh clusters`, k9s-namespace style. `0` shows every cluster's servers (the list then has a CLUSTER column), `1`-`9` one cluster's; `:cluster [name|all]` does the same. Every tsh call for a node passes its `--cluster`.
- **Proxy picker** opens with `p` or a proxy's shortcut letter. It lists each proxy with its login state (logged in / expired / no session) and latency. Keys: `<enter>` switch, `<e>` edit, `<a>` add.
- **Header**: the certificate countdown turns yellow under 30 min and red under 5 min, then offers a re-login.
- **Server list columns**: ★ favorite, name, key label (env), reachability dot.
- **Footer** key hints change with the focused pane.
- **Focus model**: the terminal captures every key, Ctrl+C included. `Ctrl+B` (`keybindings.focus`) is a tmux-style prefix. In the terminal, the prefix followed by `Tab` goes to the list, `1`-`9` (or `n`/`p`) picks a tab, and `c`, `d`, `r`, `[`, `l` and `q` close the pane, detach, reconnect, scroll, log in and quit. Each tab holds split panes (tmux-style, one session each): prefix `|`/`%` and `-`/`"` split right/below through a server picker, arrows or `o` move between panes, `z` zooms one pane, `f` toggles fullscreen (only the terminal is drawn), and `:` opens the palette. The prefix twice sends a literal Ctrl+B. `Alt+1..9` switches tabs from anywhere. On the list, `Enter` opens a session or focuses the open one (in any tab or pane), `n` (or `Shift+Enter` where the terminal reports it) opens another, `|`/`-` open the selected server as a split, and `F` goes fullscreen. Config `terminal:` has `scrollback` (default 10000), `auto_reconnect` and `mouse` (default on: drag, double-click (word) and triple-click (line) select and copy; Shift does so through a program that wants the mouse).

## Behaviour

1. `tstui` starts, reads `~/.config/tstui/config.yaml`, picks the default proxy, and checks the `tsh` login state.
2. If not logged in, it logs in with whatever came on the command line and **opens a popup for anything missing** (user, password, MFA code).
3. The header shows the project name and version, current proxy, user and roles, server version, client (tsh) version, login status with the certificate countdown, MFA type, latency, open session count, and the clusters panel.
4. The left pane lists the servers of the **current proxy**. Enter opens an SSH TTY in the main pane. The login name is picked from the allowed logins, with a popup when there is more than one.
5. The proxy picker (`p`, `:proxy`) lists every configured proxy: the config file plus the `tsh` profiles in `~/.tsh`. Selecting one switches the current proxy, logs in if needed, and reloads its server list. The left pane lists the servers of all the proxy's clusters (root and leaf), or of the one picked in the clusters panel.
6. Look and feel is k9s-like: line borders, a title on every pane, a coloured header, footer hints, skins, and a `?` help overlay.
7. Extras: search, filters and favorites; multi-session tabs; certificate expiry and health; a command palette with scp, port-forward and multi-exec.

## CLI

```
tstui [--proxy HOST:PORT|NAME] [--user NAME] [--password-stdin | --password-file PATH]
      [--password PASS]   # supported, but prints a warning (shell history, ps)
      [--mfa-token CODE]  # OTP code
      [--auth CONNECTOR] [--insecure] [--config PATH] [--skin NAME] [--version]
Env vars: TSTUI_USER, TSTUI_PASSWORD, TSTUI_MFA, TSTUI_PROXY
```

Precedence is flags, then environment, then config. `--proxy` also accepts a proxy name from the config. Missing values are asked for in the popup, never on the plain terminal.

## Architecture rules

- Go 1.27.1+ (`go.mod` says 1.27.1), `tview` + `tcell`. Stay on tagged tview releases: tview master (after v0.42.0) adds unexported methods to `Primitive`, which breaks custom wrappers such as `modalLayout` in `ui/login.go`. Use `creack/pty` for PTYs. The VT emulator is `charmbracelet/x/vt` v0.1.0 with a local patch in `third_party/charmbracelet-x-vt` (a `replace` in `go.mod`). Read `docs/terminal-decision.md` and that directory's `PATCHES.md` before you upgrade it.
- Layout:
  - `cmd/tstui`: flags and wiring
  - `internal/tsh`: the tsh wrapper
  - `internal/config`
  - `internal/auth`: credentials, then login
  - `internal/ui`: header (with the clusters panel), serverlist, clusters, proxy picker, terminal (`TerminalView`, one per session), panes (`paneView`: the active tab's split panes, fullscreen, the split picker), tabs, modals, footer
  - `internal/layout`: the pane tree of a tab (equal-size nested splits) and its geometry: rects, dividers, neighbour lookup (pure)
  - `internal/vterm`: the emulator behind a mutex, plus the key, paste and mouse encoders
  - `internal/session`: a `Session` (one PTY process with its `vterm.Terminal`), the tab `Manager` (tabs of split panes), `tsh ssh` and demo-shell spawners, stderr error classification, and the remembered logins (`$XDG_STATE_HOME/tstui/logins.json`)
  - `internal/health`: the client/server version rule (`CheckVersions`, its only home), expiry thresholds and alerts, node reachability from `tsh ls`, and the periodic proxy `Monitor`
  - `internal/palette`: the `:` command parser, completion and history (pure)
  - `internal/filter`: the server filter language (pure)
  - `internal/transfer`: `tsh scp` argv building, local path completion, the transfer log
  - `internal/portfwd`: `tsh ssh -N --forward` specs, port conflict checks, and the forward `Manager`
  - `internal/multiexec`: the concurrency-limited runner, the risky-command detector, the audit log and export
  - `scripts/`: `version.sh` (the ldflags version), `offline-build.sh`, `package.sh` (`make release`: plain binaries, `.tar.gz`, `.deb`, `SHA256SUMS`, `THIRD_PARTY_LICENSES`; Linux and macOS only)
  - `examples/`: `config.yaml` with every key and `skins/*.yaml`. Tests load both, so a new config key needs an example. Skin files live in `skins/` next to the config (`ui.LoadSkins`).
  - Feature docs for users are in `docs/features.md`. The README, `docs/architecture.md`, `docs/security.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `docs/tstui.1` and `docs/verification-checklist.md` cover the rest.
- Every `tsh` call goes through the `tsh.Client` interface. Tests use `tsh.Fake` (in-memory) or the fake tsh script `testdata/fake-tsh/tsh`, which is controlled by `FAKE_TSH_STATE` and the other knobs in its header. A test that uses the bare name `tsh` must also `t.Setenv("PATH", …)`: `exec` resolves the binary from the test process's PATH, not the child's `Environ`, so without it the real tsh runs.
- Interactive tsh commands (login, ssh, port forwards) run through `Client.StartPTY`; non-interactive ones (`tsh scp`, multi-exec's `tsh ssh host cmd`) through `Client.Exec`, always as an argv, never via `sh -c`. A UI wrapper primitive (like `modalLayout`, `viewLayout`) must forward `InputHandler`: tview v0.42 hands keys down from the root. Login is an expect-style state machine in `internal/auth`; its prompt patterns are in `auth/patterns.go` and the config can extend them (`tsh_prompts`). `tstui` opens the UI (`ui.App`). It uses a valid `~/.tsh` profile as is, and otherwise opens the k9s-style login dialog over the UI. Hidden flags: `--login-only` (UI, exit after login), `--report` (plain-text debug report), `--log-file PATH`, `--demo-shell` (a local bash in a tab instead of `tsh ssh`; `make run-demo`).
- The UI never blocks. Run `tsh` calls in goroutines and update the UI through `App.post`. Sessions report through `Options.OnChange`, which must not block: the app turns it into at most one redraw per frame. Stop the app with `App.shutdown`, never `tv.Stop` directly: `QueueUpdateDraw` waits for the event loop, so `pump` must return before tview stops.
- Tests in `session`, `vterm` and `ui` run goleak (`TestMain`). Every goroutine must end when its session or the app closes. Tests use `session.Fake` (an in-memory process) or `/bin/sh` in a real PTY.
- Builds are static (`CGO_ENABLED=0`) from vendored modules (`-mod=vendor`), so they work air-gapped. Targets: linux/amd64, linux/arm64, darwin. After changing dependencies, run `make vendor` and commit `vendor/`.
- Style: small packages, errors wrapped with context, no global state, table-driven tests.
- The `Makefile` is the source of truth for commands. Before you finish, run `make vet test lint build`. `make race` needs cgo. `make run-fake STATE=expired` runs the binary against the fake tsh.

## Security rules (non-negotiable)

- Keep passwords and OTPs in `auth.Secret`. It prints and logs as `[REDACTED]`. Call `Wipe()` as soon as the value is used. Keep secrets as `[]byte`; a `string` cannot be wiped.
- Credentials reach `tsh` only through the PTY prompt, never as argv. `tsh.SafeEnv` removes `TSTUI_PASSWORD` and `TSTUI_MFA` from every child's environment.
- Logs carry the command (through `tsh.RedactArgs`), exit code and timing only, never stdout or stderr. Error messages show tsh's stderr, which holds no secrets because none are passed in.
- The config file is mode 0600 (`config.Save` enforces it; `config.Load` warns otherwise). `--password` and loose `--password-file` permissions print warnings.
- Every new secret-handling path gets a test that captures the logs or child argv and asserts the secret is absent, like `TestCLINoSecretsReachChild`.
