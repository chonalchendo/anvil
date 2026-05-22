#!/usr/bin/env bash
# read-result.sh — print the pr_url from <worktree>/.fleet/result.json (empty when
# null) and exit 0 when the artifact parses. Exits non-zero when the file is missing
# or malformed, signalling the fleet orchestrator to fall back to
# `gh pr list --head <branch>`. The orchestrator reads .status / .blockers from the
# same file once this returns 0; the subagent's final stdout line is ignored.
#
# Usage:
#   read-result.sh <worktree>
set -euo pipefail

worktree=${1:?usage: read-result.sh <worktree>}
file="$worktree/.fleet/result.json"
[ -f "$file" ] || { echo "read-result.sh: no result at $file" >&2; exit 1; }

# jq -e fails non-zero when the JSON is malformed or .status is absent — either
# means "unparseable", so the orchestrator falls back rather than trusting it.
jq -e '.status' "$file" >/dev/null
jq -r '.pr_url // ""' "$file"
