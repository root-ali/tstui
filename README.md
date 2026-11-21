# tstui

A k9s-style terminal UI for [Teleport](https://goteleport.com) SSH. tstui wraps `tsh`. It lists the servers of a proxy and its leaf clusters, opens SSH sessions in an embedded terminal with tabs and split panes, and logs you in through a popup that handles passwords, OTP codes and security keys. It also does file transfer, port forwards and multi-exec.

```
┌─ tstui ───────────────────────────────────────────────────────────────────────────────────┐
│ Project : tstui v0.1.0       User   : ali  (roles: access,editor)    Clusters            │
│ Proxy   : tp.ast.example ●   Login  : ✔ logged in · cert 07h42m      <0> all 155         │
│ Server  : v18.10.0           MFA    : otp                            <1> teleport-ast 59 │
│ Client  : v18.11.3 (tsh)     Latency: 23ms   Sessions: 2             <2> afr 55          │
│ Cluster : teleport-ast       Filter : env=prod                       <3> hwb 41          │
├────────────────────────┬──────────────────────────────────────────────────────────────────┤
│ Servers (afr) [12/42]  │ [1:web-01 ●] [2:db-03 +1 ●]                                      │
│ ───────────────────────│ ┌──────────────────────────────────────────────────────────────┐ │
│ ★ web-01   prod  ●     │ │ ali@web-01:~$ uptime                                         │ │
│ ★ web-02   prod  ●     │ │  10:42:01 up 41 days,  2 users,  load average: 0.08          │ │
│   db-03    prod  ●     │ │ ali@web-01:~$ _                                              │ │
│   cache-01 stage ○     │ └──────────────────────────────────────────────────────────────┘ │
├────────────────────────┴──────────────────────────────────────────────────────────────────┤
│ <enter>connect </>filter <f>fav <p>proxies <:>cmd <ctrl-b>focus <alt-1..9>tab <?>help <q> │
└───────────────────────────────────────────────────────────────────────────────────────────┘
```

The login popup:

```
          ┌──────────────────────── <Login> ─────────────────────────┐
          │                                                          │
          │                    Log in to Teleport                    │
          │                                                          │
          │  Proxy:    tp.ast.example.com:443                        │
          │  User:     ali                                           │
          │  Password: **********                                    │
          │  MFA code: if asked                                      │
          │                                                          │
          │                   [ Login ]  [ Cancel ]                  │
          │                                                          │
          │  Tab next field · Enter log in · Esc cancel              │
          │                                                          │
          └──────────────────────────────────────────────────────────┘
```

Two sessions side by side in fullscreen (`ctrl-b |`, then `ctrl-b f`):

```
ali@web-01:~$ htop                          │ali@db-03:~$ tail -f /var/log/syslog
  1  [|||||          12.4%]                 │Nov 19 10:42:07 db-03 systemd[1]: Started
  2  [|||             7.9%]                 │Nov 19 10:42:09 db-03 CRON[2211]: (root)
  Mem[||||||||  1.21G/7.7G]                 │Nov 19 10:42:11 db-03 postgres[903]: LOG:
                                            │
```

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [Command-line flags](#command-line-flags)
- [Environment variables](#environment-variables)
- [Configuration](#configuration)
- [Keys](#keys)
- [Troubleshooting](#troubleshooting)
- [More documentation](#more-documentation)

## Install

tstui needs **tsh** on your `PATH`, in the major version of your Teleport cluster or one major version older. tstui supports tsh v16 and newer and has been tested with v18. Get tsh from <https://goteleport.com/download/>, Teleport's apt or yum repository (package `teleport`), or `brew install teleport`. Check it with `tsh version`.

tstui itself is a single static binary with no other dependencies. Each [release](https://github.com/root-ali/tstui/releases) has builds for Linux (amd64, arm64) and macOS (Intel `amd64`, Apple silicon `arm64`). Windows is not supported yet, because the embedded terminal and the login need a Unix pseudo-terminal.

**Plain binary** (Linux or macOS): download the file for your platform, for example `tstui_0.1.0_linux_amd64`, then:

```sh
sudo install -m 0755 tstui_0.1.0_linux_amd64 /usr/local/bin/tstui
```

**Debian or Ubuntu** (amd64 or arm64):

```sh
sudo apt install ./tstui_0.1.0_amd64.deb
```

**Linux or macOS** (tarball, with the man page, examples and licenses): `linux_amd64`, `linux_arm64`, `darwin_amd64` or `darwin_arm64`.

```sh
tar xzf tstui_0.1.0_linux_amd64.tar.gz
sudo install -m 0755 tstui_0.1.0_linux_amd64/tstui /usr/local/bin/tstui
```

Check the download against `SHA256SUMS` with `sha256sum -c --ignore-missing SHA256SUMS`. On macOS, a binary downloaded with a browser is quarantined. Clear the flag with `xattr -d com.apple.quarantine /usr/local/bin/tstui`.

**From source** (Go 1.27.1 or newer, no network needed, because dependencies are vendored):

```sh
make build            # bin/tstui
make release          # dist/: binaries, .tar.gz, .deb and SHA256SUMS
```

## Quick start

```sh
tstui --proxy tp.ast.example.com --user ali
```

1. tstui checks the tsh login for that proxy. If you are already logged in (with `tsh login` or an earlier tstui run), the server list appears at once.
2. Otherwise the login popup opens. Type your password, then the OTP code, or touch your security key when asked.
3. Pick a server with the arrow keys (or `j`/`k`) and press `enter`. If your role allows several logins, tstui asks which one and remembers the answer.
4. You are in the remote shell. Every key goes to it, `ctrl-c` included. Press `ctrl-b tab` to go back to the list, and `enter` on another server opens a second tab.
5. Press `?` for every key, `:` for the command palette, `q` on the list to quit.

To keep proxies and favorites between runs, copy [`examples/config.yaml`](examples/config.yaml) to `~/.config/tstui/config.yaml`, edit it, and run `chmod 600` on it. Then plain `tstui` starts on `default_proxy`, and `p` switches between proxies.

To try the UI without a cluster: `make run-demo` runs tstui against a fake tsh, with a local shell in place of SSH.

## Command-line flags

| Flag | Meaning |
|---|---|
| `--proxy HOST:PORT\|NAME` | Teleport proxy address, or a proxy `name` from the config. Port 443 is assumed when none is given. |
| `--user NAME` | Teleport user |
| `--password-stdin` | Read the password from stdin: `pass show teleport \| tstui --password-stdin` |
| `--password-file PATH` | Read the password from a file. tstui warns unless the file is mode 0600. |
| `--password PASS` | The password. Works, but warns: it shows in shell history and `ps`. Prefer the two flags above. |
| `--mfa-token CODE` | OTP code. Codes expire within a minute, so this is mainly for scripts. |
| `--auth CONNECTOR` | Auth connector, as for `tsh --auth` (for example a SAML or OIDC connector name) |
| `--insecure` | Skip TLS verification of the proxy, for test clusters with self-signed certificates only |
| `--config PATH` | Config file (default `$XDG_CONFIG_HOME/tstui/config.yaml`, which is `~/.config/tstui/config.yaml`) |
| `--skin NAME` | `default`, `light`, `high-contrast`, or a skin file's name |
| `--version` | Print the version, commit, build date and platform |

Only one of `--password-stdin`, `--password-file` and `--password` may be given. Anything needed to log in that is not given is asked for in the popup, never on the plain terminal.

Debugging flags, hidden from `--help`:

| Flag | Meaning |
|---|---|
| `--report` | Print what tstui finds out (tsh version, proxy, login, clusters, servers) as plain text, without the UI |
| `--log-file PATH` | Write a debug log (mode 0600). It records commands with secrets redacted, exit codes and timings, never output. |
| `--login-only` | Open the UI, log in if needed, print the result and exit |
| `--demo-shell` | A local bash in a tab, and `enter` opens bash instead of `tsh ssh` |

Exit status: 0 success, 1 error, 2 usage error, 3 internal error (report on stderr), 128+N when stopped by signal N.

## Environment variables

| Variable | Same as |
|---|---|
| `TSTUI_PROXY` | `--proxy` |
| `TSTUI_USER` | `--user` |
| `TSTUI_PASSWORD` | the password |
| `TSTUI_MFA` | `--mfa-token` |

Flags win over environment variables, and environment variables over the config file. tstui removes `TSTUI_PASSWORD` and `TSTUI_MFA` from the environment of every process it starts, tsh included. `XDG_CONFIG_HOME` and `XDG_STATE_HOME` move the config and state directories.

## Configuration

`~/.config/tstui/config.yaml`, mode 0600. tstui warns when the file is readable by others. Every key is optional, and an unknown key is an error, so typos do not pass silently. [`examples/config.yaml`](examples/config.yaml) shows every key with comments.

| Key | Default | Meaning |
|---|---|---|
| `default_proxy` | | `name` of the proxy to start on. Without it: `--proxy`, `$TSTUI_PROXY`, or tsh's current profile. |
| `proxies[].name` | required | Short label: letters, digits, `.`, `_`, `-`. Unique. |
| `proxies[].addr` | required | `host[:port][,sshport]`, without `https://` |
| `proxies[].shortcut` | | One lowercase letter that switches to this proxy |
| `proxies[].user` | | Teleport user for this proxy |
| `proxies[].auth` | | Auth connector (`tsh --auth`) |
| `favorites.<proxy name>` | | List of hostnames pinned at the top. `f` edits it. |
| `skin` | `default` | `default`, `light`, `high-contrast`, or a skin file's name |
| `keybindings.focus` | `Ctrl-B` | The prefix key in the terminal: `Ctrl-<letter>` or `Alt-<letter>` |
| `terminal.scrollback` | `10000` | Lines kept per session, 0 to 1000000 |
| `terminal.auto_reconnect` | `false` | Restart a session that dropped (non-zero exit without a tsh error, or host unreachable) |
| `terminal.mouse` | `true` | Mouse selection (drag, double-click a word, triple-click a line; copied to the clipboard), wheel and clicks. Hold Shift to select through a program that wants the mouse. Set `false` to use your terminal's own selection instead. |
| `tsh_prompts.<kind>` | | Extra regular expressions for tsh's login prompts and errors. Kinds: `password`, `otp`, `otp_or_tap`, `tap`, `pin`, `choice`, `key_tapped`, `sso_link`, `locked`, `connector_not_found`, `wrong_credentials`, `tls`, `network`. |
| `port_forwards[].name` | required | Label in the `:pf` view, unique |
| `port_forwards[].host` | required | Server hostname |
| `port_forwards[].forward` | required | `[bind:]localport:remotehost:remoteport`, as for `ssh -L` |
| `port_forwards[].login` | last used | SSH login |
| `port_forwards[].proxy` | current | Proxy name or address |
| `port_forwards[].cluster` | where the host is | Teleport cluster of the node |
| `exec.concurrency` | `5` | Servers a multi-exec runs on at once, 1 to 50 |
| `exec.timeout` | `60s` | Time limit per server |

tstui rewrites the config when you star a server, save a port forward, or pick a skin with `:skin`. Comments are lost then. The first save in each run copies the previous file to `config.yaml.bak`.

### Skins

Besides the three built-in skins, tstui loads every `~/.config/tstui/skins/<name>.yaml`. A skin file names a built-in `base` and changes any of its colours:

```yaml
base: default
colors:
  border: "#1f6f8b"
  border_focus: "#4fd1c5"
  table_cursor_bg: "#4fd1c5"
  error: crimson
```

Colours are W3C names, `#rrggbb`, or `default` (the terminal's own colour). The keys are `bg`, `fg`, `border`, `border_focus`, `title`, `counter`, `info_label`, `info_value`, `menu_key`, `menu_text`, `table_fg`, `table_header`, `table_cursor_fg`, `table_cursor_bg`, `dialog_fg`, `dialog_label`, `dialog_field`, `dialog_placeholder`, `button_fg`, `button_bg`, `button_focus_fg`, `button_focus_bg`, `ok`, `warn`, `error`, `info`, `logo_from` and `logo_to`. [`examples/skins/`](examples/skins/) has three complete skins: `ocean`, `mono` and `paper`. A file with an error is skipped with a warning at start.

### Files tstui writes

| File | What |
|---|---|
| `~/.config/tstui/config.yaml` (+ `.bak`) | favorites, saved port forwards, skin |
| `~/.local/state/tstui/logins.json` | the SSH login last used per server |
| `~/.local/state/tstui/servers/` | the last good server list per proxy, shown marked STALE when the proxy is unreachable |
| `~/.local/state/tstui/transfers.log` | finished file transfers |
| `~/.local/state/tstui/audit.log` | every multi-exec: hosts, command, results; never output |

All are mode 0600 in 0700 directories. tstui never writes passwords or OTP codes anywhere. tsh keeps its certificates in `~/.tsh` as usual.

## Keys

The terminal takes every key. Use the prefix (`ctrl-b`) to reach tstui from inside a session. Press `?` in tstui for the full list.

**Server list**

| Key | Action |
|---|---|
| `enter` | Connect, or show the server's open session |
| `n` | Open another session to the same server |
| `\|` / `-` | Open the server in a new pane right of / below the focused pane |
| `j` `k` `↑` `↓`, `g` `G`, `pgup` `pgdn` | Move |
| `/` | Filter: `web`, `!web`, `env=prod`, `env!=prod`, `env=` |
| `f` | Toggle favorite |
| `space` / `a` | Mark a server / all shown servers (for `:exec`) |
| `u` / `d` | Upload to / download from the server |
| `N` / `A` / `C` | Sort by name / address / cluster (again reverses) |
| `0`, `1`-`9` | All clusters, or one cluster |
| `p` | Proxy picker |
| `l` / `L` / `o` | Log in / log in again / log out |
| `r` | Refresh |
| `F` | Fullscreen terminal |
| `:` | Command palette |
| `?` | Help |
| `q`, `ctrl-c` | Quit (asks first while sessions or port forwards are open) |

**In the terminal, after the prefix `ctrl-b`**

| Key | Action |
|---|---|
| `tab` | Back to the server list |
| `1`-`9`, `n` / `p` | Tab 1-9 / next / previous (`alt-1`..`alt-9` work without the prefix) |
| `\|` or `%`, `-` or `"` | Split right / below (pick a server) |
| arrows, `o` | Move between panes |
| `z` / `f` | Zoom the pane / fullscreen |
| `[` | Scroll mode (`pgup`/`pgdn`, `q` leaves). The mouse wheel scrolls too. |
| `c` / `d` / `r` | Close the pane / detach / reconnect |
| `l` / `q` | Log in / quit |
| `:` / `?` | Palette / help |
| `ctrl-b` | Send a literal `ctrl-b` |

**Command palette** (`:`; `tab` completes, `↑`/`↓` history): `:proxy`, `:cluster`, `:login`, `:logout`, `:ssh <host>`, `:sessions`, `:split`, `:vsplit`, `:fullscreen`, `:zoom`, `:filter`, `:sort`, `:fav`, `:refresh`, `:skin`, `:scp`, `:pf`, `:exec`, `:help`, `:quit`.

[docs/features.md](docs/features.md) explains each feature in detail.

## Troubleshooting

Start with `tstui --report --proxy <proxy>`. It prints the tsh version, whether the proxy answers, its auth type and MFA setting, your login state and the servers, all without the UI. Add `--log-file /tmp/tstui.log` to a normal run to record which tsh commands ran and how they ended.

**"tsh, Teleport's command-line client, is not on your PATH"**
Install tsh (see [Install](#install)), then check that `tsh version` works in the same shell.

**"tsh vN is not supported" or "tsh vN untested", or a version warning in the header**
A Teleport server supports tsh of its own major version and one major version older, not newer. Install the tsh that matches the cluster: `tstui --report` shows both versions.

**"wrong user name or password"** (tsh: `invalid username or password`)
Check the user name in the popup, in `--user` and in the config. Teleport users are case-sensitive. If you log in through SSO, local passwords do not apply. Set `--auth` or the proxy's `auth:` to your connector.

**"wrong OTP code"** (tsh: `invalid username, password or second factor`)
OTP codes depend on the clock. Check that your phone's time is set automatically, and wait for a fresh code instead of reusing one.

**"user is locked"** (tsh: `too many incorrect attempts, please try again later`, or `lock targeting … is in force`)
Teleport locks a user for a while after several failed logins. Wait, or ask a Teleport admin to lift the lock.

**"auth connector not found"**
The `--auth` value or the proxy's `auth:` key names a connector the cluster does not have. Remove it to use the cluster's default, or ask an admin for the right name. `tstui --report` shows the auth type the proxy advertises.

**"TLS error"**
The proxy's certificate is not trusted on this machine: it is self-signed, or a TLS-inspecting proxy sits in between. Install the CA in the system trust store. `--insecure` works for test clusters only.

**"cannot reach the proxy"** (tsh: `Teleport proxy not available at …`)
Check the address and port: the proxy's web port, usually 443 or 3080, not the SSH port 3023. Then check DNS and the VPN. `curl https://<proxy>/webapi/ping` should return JSON.

**The popup says "touch your security key" and nothing happens**
Check that the key is plugged in and that `tsh login --proxy=<proxy>` works in a plain terminal. On Linux, the key's device must be readable by your user (FIDO udev rules). A login that gets no answer stops after 3 minutes ("login timed out").

**SSO login (OIDC, SAML, GitHub)**
The popup shows the login URL. tsh opens the browser if it can. Otherwise open the link yourself, and the popup continues when the browser login finishes.

**"access denied" when connecting to a server**
Your roles do not allow that login on that server. Pick another login (tstui asks when you have several), or request access. **"this server requires MFA for every session"** means you must touch your key or enter a code when the session starts.

**"certificate has expired"**
Press `L` to log in again. Open sessions are left running.

**The server list says STALE**
The proxy could not be reached, so tstui shows the last list it got, with its age. `r` tries again.

**Login prompts are not recognised (the popup stays on "Logging in…")**
A different tsh version may word a prompt differently. Run `tsh login --proxy=<proxy>` once by hand, copy the prompt text, and add a pattern under `tsh_prompts` in the config. Please also report the prompt text.

**The terminal looks wrong after tstui exits**
tstui restores the terminal even after a crash, but if something went wrong, `reset` fixes it.

## More documentation

- [docs/features.md](docs/features.md): tabs and panes, clusters, health and expiry, palette, filters, file transfer, port forwards, multi-exec
- [docs/architecture.md](docs/architecture.md): how the code is organised
- [docs/security.md](docs/security.md): how credentials are handled and what is written where
- [docs/tsh-notes.md](docs/tsh-notes.md): what tstui relies on from tsh, and which parts are verified
- [docs/verification-checklist.md](docs/verification-checklist.md): manual tests against real clusters
- [CONTRIBUTING.md](CONTRIBUTING.md): building, testing, conventions
- [CHANGELOG.md](CHANGELOG.md): release notes
- `man tstui` (installed by the `.deb`; `docs/tstui.1` in the source)

## License

tstui is licensed under the [Apache License 2.0](LICENSE). See [NOTICE](NOTICE). Release archives and the `.deb` include `THIRD_PARTY_LICENSES` with the licenses of the Go standard library and every module linked into the binary.
