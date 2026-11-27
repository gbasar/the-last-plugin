# The Last Plugin (`lplug`)

Register one plugin once. From then on you add hooks, rules, knowledge bases
and skills by dropping files in `~/.config/lplug/`. You never register another plugin.

- **Sources in any language:** executables, `.md`/`.txt` text, or rule files in INI, TOML, JSON or HOCON.
- **Rules:**
  - filters: `tool`, `when`, `unless`, `every`, `once`
  - actions: `text`, `file`, `ask`, `run`, `confirm`, `block`
- **Layers:** your personal layer, plus a project layer at `<repo>/.config/lplug`. The project wins, and env vars beat both.
- **Knowledge bases**, including script libraries, listed for the agent at session start.
- **Skills** in `skills.d/`, linked into the agent's skills folder automatically.
- **Slash commands** in `commands.d/` that can call each other: a step that is just `/other-command` runs its steps there.
- **Toggles:**
  - `lplug off ponytail`, `lplug off @work`, `lplug on 'git-*'`
  - in Claude Code, `! lplug off ponytail` doesn't spend a turn
- **Agent-neutral core:** Claude Code is the first adapter. See [SPEC.md](SPEC.md) for the contract.
- **XDG everywhere**, one static binary per platform, about 1 ms per hook when nothing is configured.

## Install (Claude Code)

```sh
bash install.sh                         # builds from source, installs plugin + habits
# or by hand:
./build.sh --install                    # build for this machine, link ~/.local/bin/lplug
claude --plugin-dir ./agents/claude-code
```

lplug builds with Go's own build system: `go build`, with dependencies in
`go.mod` and their checksums in `go.sum` (like a Maven `pom.xml`). `build.sh`
just wraps `go build`; with no options it builds macOS, Linux and Windows on
amd64 and arm64. Without Go, `install.sh` uses the prebuilt binaries.

**Which Go:** `$LPLUG_GO`, else `go` on PATH, else `~/.local/share/go/bin/go`.

**Where libraries come from (e.g. a Nexus Go proxy):** Go's own setting,
stored in `~/.config/go/env`. Set it once:

```sh
go env -w GOPROXY=https://<nexus>/repository/<go-proxy>/
go env -w GOSUMDB=off      # only if sum.golang.org is blocked; go.sum still checks
go env -w GOTOOLCHAIN=local  # never download a newer Go toolchain
```

## Quick start

```sh
mkdir -p ~/.config/lplug/session-start.d ~/.config/lplug/rules.d
echo 'Code very efficiently.' > ~/.config/lplug/session-start.d/50-ponytail.md

cat > ~/.config/lplug/rules.d/work.ini <<'EOF'
[tests]
event = tool_after
tool  = Bash
every = 5
text  = Run the tests before saying you're done.

[no-main]
event = tool_before
when  = git push.*\bmain\b
block = Don't push to main; open a PR.
tags  = strict

[kb]
event = session_start, subagent_start
run   = lplug kb render
EOF

lplug status
lplug help          # topics: sources rules scripts events toggles settings kb skills
```

## Layout

```
cmd/lplug/              entry point
internal/lplug/         everything, one package:
  config.go             layers, settings.ini, XDG paths, the INI parser
  event.go agent.go     neutral events; the Claude Code adapter
  rules.go              rule formats and matching
  run.go                the dispatcher: collect, run, merge
  state.go              lock-free counters, toggles
  kb.go skills.go       knowledge bases, skills.d
  commands.go           commands.d: composable slash commands
  cli.go help.go        commands and help
agents/claude-code/     the plugin: manifest, hooks.json, /lplug command, binaries
```

## Extending

- **A command:** add one entry to `commands` in `cli.go`; help lists it automatically.
- **An agent:** implement `Adapter` in `agent.go` and add it to `adapters`.
- **Tests:** `go test ./...`

## Next version

- Approving project layers before they load (`lplug trust`)
- `--session` toggles
- `intent.log`
- `lplug edit` / `check` / `list`
