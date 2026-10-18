#!/usr/bin/env bash
# Installs the runtime dependencies AgentKit skills expect, without the ak CLI or an AgentKit login.
#
# What lands where:
#   ~/.claude/skills/.venv      shared Python venv; the kit's hooks point skill scripts at it
#   .agentkit/.../node_modules  per-skill npm packages (wiped and re-installed with every setup)
#   ~/.local/bin                repomix, agent-browser, mmdc via `npm install -g` (npm prefix)
#   ~/.agent-browser/browsers   the Chromium agent-browser drives
# Everything outside .agentkit/ survives a kit upgrade and is only installed when missing.
#
# Usage: scripts/agentkit-deps.sh   (also run at the end of scripts/agentkit-setup.sh)
set -uo pipefail
cd "$(dirname "$0")/.."

SKILLS=".agentkit/ak-engineer-kit/ak-engineer/skills"
VENV="$HOME/.claude/skills/.venv"
FAILED=""

[ -d "$SKILLS" ] || { echo "kit not installed: run make agentkit-setup KIT=<tar.gz> first" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 is required for skill scripts" >&2; exit 1; }
command -v npm     >/dev/null || { echo "npm is required for skill packages and CLIs" >&2; exit 1; }

# Keep going past a failed step so one broken optional dependency does not hide the rest.
run() { "$@" || FAILED="$FAILED\n  $*"; }

echo "▸ Python venv for skill scripts → $VENV"
[ -x "$VENV/bin/python3" ] || run python3 -m venv "$VENV"
run "$VENV/bin/pip" install -q --upgrade pip
# skill-creator pins PyYAML by hash; the rest are ranges.
run "$VENV/bin/pip" install -q --require-hashes -r "$SKILLS/ak-skill-creator/scripts/requirements.txt"
for r in ak-mcp-builder ak-repomix ak-cti-expert; do
  run "$VENV/bin/pip" install -q -r "$SKILLS/$r/scripts/requirements.txt"
done

echo "▸ Per-skill npm packages"
for d in ak-markdown-novel-viewer ak-stitch/scripts ak-show-off/scripts; do
  [ -f "$SKILLS/$d/package.json" ] || continue
  run npm install --prefix "$SKILLS/$d" --no-audit --no-fund --loglevel=error
done

echo "▸ Global CLIs → $(npm config get prefix)/bin"
for pkg in repomix agent-browser @mermaid-js/mermaid-cli; do
  bin="${pkg##*/}"; [ "$bin" = mermaid-cli ] && bin=mmdc
  command -v "$bin" >/dev/null || run npm install -g --no-audit --no-fund --loglevel=error "$pkg"
done
if command -v agent-browser >/dev/null && [ -z "$(ls -A "$HOME/.agent-browser/browsers" 2>/dev/null)" ]; then
  run agent-browser install
fi

if [ -n "$FAILED" ]; then
  printf "✗ Some AgentKit dependencies failed to install:%b\n" "$FAILED" >&2
  exit 1
fi
echo "✓ AgentKit skill dependencies installed."
