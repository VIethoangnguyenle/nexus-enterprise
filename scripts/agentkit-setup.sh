#!/usr/bin/env bash
# Installs the AgentKit engineer kit (ak-engineer) for this checkout.
#
# The kit is a paid, licensed product and this repository is public, so the kit itself is never
# committed. Each machine extracts its own licensed copy into .agentkit/ (git-ignored); the
# committed .claude/settings.json already declares the marketplace at that relative path and
# enables the plugin, so Claude Code picks up its skills, agents, rules and hooks on next start.
#
# Usage: scripts/agentkit-setup.sh <path/to/ak-engineer-kit-X.Y.Z.tar.gz>
#        make agentkit-setup KIT=<path/to/ak-engineer-kit-X.Y.Z.tar.gz>
set -euo pipefail
cd "$(dirname "$0")/.."

TARBALL="${1:-${KIT:-}}"
DEST=".agentkit/ak-engineer-kit"

if [ -z "$TARBALL" ] || [ ! -f "$TARBALL" ]; then
  echo "usage: $0 <path/to/ak-engineer-kit-X.Y.Z.tar.gz>" >&2
  exit 2
fi
command -v node   >/dev/null || { echo "node is required — kit hooks are Node scripts" >&2; exit 1; }
command -v claude >/dev/null || { echo "claude CLI is required to register the plugin" >&2; exit 1; }

echo "▸ Extracting $TARBALL → $DEST"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
# --exclude drops macOS AppleDouble files (._*) that the archive carries.
tar -xzf "$TARBALL" -C "$STAGE" --exclude='._*' --no-same-owner 2>/dev/null
ROOT="$(find "$STAGE" -mindepth 1 -maxdepth 1 -type d | head -1)"
if [ ! -f "$ROOT/.claude-plugin/marketplace.json" ] || [ ! -f "$ROOT/ak-engineer/hooks/hooks.json" ]; then
  echo "not an AgentKit engineer kit: missing .claude-plugin/marketplace.json or ak-engineer/hooks/hooks.json" >&2
  exit 1
fi
rm -rf "$DEST"
mkdir -p "$(dirname "$DEST")"
mv "$ROOT" "$DEST"

echo "▸ Registering plugin ak-engineer@agentkit-local (project scope)"
claude plugin marketplace update agentkit-local >/dev/null 2>&1 \
  || claude plugin marketplace add "./$DEST" --scope project
claude plugin install ak-engineer@agentkit-local --scope project -y
# `marketplace add` records an absolute path; keep the committed setting machine-independent.
node -e '
  const fs = require("fs"), f = ".claude/settings.json";
  const s = JSON.parse(fs.readFileSync(f, "utf8"));
  s.extraKnownMarketplaces["agentkit-local"].source.path = "./" + process.argv[1];
  fs.writeFileSync(f, JSON.stringify(s, null, 2) + "\n");
' "$DEST"

# Plugins carry no rules/. Claude Code loads .claude/rules/*.md as project memory and the kit's
# hooks look for its rules there, so link them in. .claude/rules/ is git-ignored for this reason.
echo "▸ Linking kit rules into .claude/rules/"
mkdir -p .claude/rules
for f in "$DEST"/ak-engineer/rules/*.md; do
  ln -sfn "../../$f" ".claude/rules/$(basename "$f")"
done

# The plugin system wires hooks but not the statusline. The statusline needs an absolute path,
# so it goes in the machine-local settings file rather than the committed one.
echo "▸ Wiring the AgentKit statusline into .claude/settings.local.json"
node -e '
  const fs = require("fs"), f = ".claude/settings.local.json";
  const s = fs.existsSync(f) ? JSON.parse(fs.readFileSync(f, "utf8")) : {};
  s.statusLine = { type: "command", command: "node \"" + process.argv[1] + "\"", padding: 0 };
  fs.writeFileSync(f, JSON.stringify(s, null, 2) + "\n");
' "$PWD/$DEST/ak-engineer/ak-engineer-statusline.cjs"

echo "✓ AgentKit installed. Restart Claude Code; skills appear as ak-engineer:<skill>."
