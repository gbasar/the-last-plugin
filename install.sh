#!/usr/bin/env bash
# Purpose: install lplug for Claude Code on this machine: the plugin, the
# `lplug` command, and the example habits (without overwriting your files).
# Usage:   bash install.sh        (safe to re-run; works from any shell)
# Tested:  bash 5.2 on Linux; written for bash 3.2 too (macOS /bin/bash)
set -euo pipefail   # stop on any error, unset variable, or failure inside a pipe

REPO_DIR=$(cd "$(dirname "$0")" && pwd)
CONFIG_DIR=${LPLUG_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/lplug}
BIN_DIR=${XDG_BIN_HOME:-$HOME/.local/bin}
EXAMPLES_DIR="$REPO_DIR/examples/habits"
MARKETPLACE_NAME="the-last-plugin"
PLUGIN_ID="the-last-plugin@the-last-plugin"
SHIM="$REPO_DIR/agents/claude-code/bin/lplug"

step() { printf '\n==> %s\n' "$1"; }

step "Checking prerequisites"
if ! command -v claude >/dev/null; then
    echo "Claude Code's 'claude' command isn't on PATH. Install Claude Code first." >&2
    exit 1
fi
# Build from source with Go (build.sh finds it: $LPLUG_GO, PATH, ~/.local/share/go).
# Exit 3 means no Go; then fall back to the prebuilt binary, if there is one.
build_status=0
"$REPO_DIR/build.sh" --local || build_status=$?
if [[ $build_status -eq 3 ]]; then
    echo "No Go found, so using the prebuilt binary. Set LPLUG_GO to build from source."
elif [[ $build_status -ne 0 ]]; then
    echo "Building from source failed (see above). Check 'go env GOPROXY'." >&2
    exit 1
fi
if ! "$SHIM" version >/dev/null 2>&1; then
    echo "No lplug binary for $(uname -s)/$(uname -m), and no Go to build one." >&2
    exit 1
fi
echo "lplug binary: $("$SHIM" version)"

step "Linking the lplug command into $BIN_DIR"
mkdir -p "$BIN_DIR"
ln -sf "$SHIM" "$BIN_DIR/lplug"
case ":$PATH:" in
    *":$BIN_DIR:"*) echo "linked: $BIN_DIR/lplug" ;;
    *) echo "linked: $BIN_DIR/lplug - but $BIN_DIR is not on your PATH yet; add it to use 'lplug' directly." ;;
esac

step "Copying example habits into $CONFIG_DIR (existing files are kept)"
mkdir -p "$CONFIG_DIR"
copied_count=0
kept_count=0
# Walk the examples file by file, so one existing file never blocks the rest.
while IFS= read -r -d '' example_file; do
    relative_path=${example_file#"$EXAMPLES_DIR"/}
    destination="$CONFIG_DIR/$relative_path"
    if [[ -e "$destination" ]]; then
        kept_count=$((kept_count + 1))
        continue
    fi
    mkdir -p "$(dirname "$destination")"
    cp "$example_file" "$destination"
    chmod go-w "$destination"   # lplug ignores files others can write
    copied_count=$((copied_count + 1))
done < <(find "$EXAMPLES_DIR" -type f ! -name 'claude-settings.json' -print0)
echo "copied: $copied_count   kept (already there): $kept_count"

step "Installing the Claude Code plugin"
if claude plugin marketplace list 2>/dev/null | grep --quiet "$MARKETPLACE_NAME"; then
    echo "marketplace already added; refreshing it"
    claude plugin marketplace update "$MARKETPLACE_NAME" >/dev/null || true
else
    claude plugin marketplace add "$REPO_DIR"
fi
# `install` refuses an installed plugin, so fall back to `update` on re-runs.
# wiki is a separate plugin (/wiki:ingest, /wiki:query) so teammates can install it alone.
for plugin_id in "$PLUGIN_ID" "wiki@$MARKETPLACE_NAME"; do
    claude plugin install "$plugin_id" 2>/dev/null || claude plugin update "$plugin_id"
done

step "Done. Still up to you:"
cat <<EOF
  1. Fill in <dev-cli> and <nexus-url> in
       $CONFIG_DIR/session-start.d/00-work-safety.md
  2. Point the knowledge bases at your real folders:
       $CONFIG_DIR/knowledge-bases.ini
  3. Optional: merge the permissions from
       $EXAMPLES_DIR/claude-settings.json
     into ~/.claude/settings.json (not done automatically, so nothing of yours is overwritten).
  4. Not at work? Turn the work rules off:   lplug off @work
  5. Check everything:   lplug status
  6. Start a new Claude Code session; the habits load at startup.
EOF
