#!/usr/bin/env bash
# write-result.sh — write <worktree>/.fleet/result.json, the structured outcome a
# dispatched completing-issue subagent returns to the fleet orchestrator. The
# orchestrator reads this file (via read-result.sh), not the agent's final stdout
# line, which drifts into narrative at poll-loop exit. jq builds the JSON so a null
# pr_url and an array-typed blockers field are guaranteed regardless of shell quoting.
#
# Usage:
#   write-result.sh --worktree <path> --issue <id> --branch <branch> \
#                   --status <pr_opened|blocked|abandoned> [--pr-url <url>] [--blocker <msg>]...
set -euo pipefail

worktree="" issue="" branch="" status="" pr_url=""
blockers=()
while [ $# -gt 0 ]; do
    case "$1" in
        --worktree) worktree=$2; shift 2 ;;
        --issue)    issue=$2;    shift 2 ;;
        --branch)   branch=$2;   shift 2 ;;
        --status)   status=$2;   shift 2 ;;
        --pr-url)   pr_url=$2;   shift 2 ;;
        --blocker)  blockers+=("$2"); shift 2 ;;
        *) echo "write-result.sh: unknown argument: $1" >&2; exit 2 ;;
    esac
done

case "$status" in
    pr_opened|blocked|abandoned) ;;
    *) echo "write-result.sh: --status must be pr_opened|blocked|abandoned (got: ${status:-<empty>})" >&2; exit 2 ;;
esac
for f in worktree issue branch; do
    if [ -z "${!f}" ]; then echo "write-result.sh: --$f is required" >&2; exit 2; fi
done

if [ ${#blockers[@]} -eq 0 ]; then
    blockers_json='[]'
else
    blockers_json=$(printf '%s\n' "${blockers[@]}" | jq -R . | jq -s .)
fi

mkdir -p "$worktree/.fleet"
jq -n \
    --arg issue_id "$issue" \
    --arg branch "$branch" \
    --arg worktree_path "$worktree" \
    --arg pr_url "$pr_url" \
    --arg status "$status" \
    --argjson blockers "$blockers_json" \
    '{issue_id: $issue_id, branch: $branch, worktree_path: $worktree_path,
      pr_url: (if $pr_url == "" then null else $pr_url end),
      status: $status, blockers: $blockers}' \
    > "$worktree/.fleet/result.json"
