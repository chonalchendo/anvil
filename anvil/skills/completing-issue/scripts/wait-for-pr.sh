#!/usr/bin/env bash
# Out-of-band PR poller: blocks until a terminal state is reached, then emits
# one JSON result. Invoke once from an agent tool call rather than polling
# in-loop, which burns tokens on every iteration of the LLM context.
#
# Terminal states:
#   merged              — PR was merged
#   closed              — PR closed without merge
#   review_blocked      — unresolved review threads exist (CodeRabbit or human)
#   ci_failed           — at least one required CI check failed
#   timeout             — configurable limit reached with no terminal state
#
# Usage: wait-for-pr.sh --pr <number> [--repo <owner/repo>] [--timeout <seconds>] [--interval <seconds>]
set -euo pipefail

# Defaults
POLL_INTERVAL=30
TIMEOUT=900  # 15 minutes — aligns with CodeRabbit rate-limit-fallback policy
PR_NUMBER=""
REPO=""

usage() {
    cat >&2 <<'EOF'
Usage: wait-for-pr.sh --pr <number> [--repo <owner/repo>] [--timeout <seconds>] [--interval <seconds>]

Poll a PR until one terminal state is reached, then emit a single JSON result.

Options:
  --pr <number>        PR number to poll (required)
  --repo <owner/repo>  GitHub repository (default: current repo from git remote)
  --timeout <seconds>  Timeout in seconds (default: 900 = 15 min)
  --interval <seconds> Poll interval in seconds (default: 30)

Output fields (JSON):
  state                merged | closed | review_blocked | ci_failed | timeout
  merged               true | false
  ci_conclusion        success | failure | pending | skipped | null
  review_blockers_count number of unresolved review threads
  timed_out            true | false
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pr) PR_NUMBER="$2"; shift 2 ;;
        --repo) REPO="$2"; shift 2 ;;
        --timeout) TIMEOUT="$2"; shift 2 ;;
        --interval) POLL_INTERVAL="$2"; shift 2 ;;
        --help|-h) usage ;;
        *) echo "Unknown option: $1" >&2; usage ;;
    esac
done

if [[ -z "$PR_NUMBER" ]]; then
    echo "Error: --pr is required" >&2
    usage
fi
# Caller-supplied numerics feed arithmetic under set -e; a bad value would exit
# silently, so reject it with a usable message instead.
[[ "$TIMEOUT" =~ ^[0-9]+$ ]] || { echo "Error: --timeout must be integer seconds" >&2; exit 1; }
[[ "$POLL_INTERVAL" =~ ^[0-9]+$ ]] || { echo "Error: --interval must be integer seconds" >&2; exit 1; }

# Resolve repo from git remote when not supplied.
if [[ -z "$REPO" ]]; then
    REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null) || {
        echo "Error: could not resolve repo — pass --repo owner/repo" >&2
        exit 1
    }
fi
OWNER="${REPO%%/*}"
REPO_NAME="${REPO##*/}"

emit() {
    local state="$1" merged="$2" ci_conclusion="$3" review_blockers="$4" timed_out="$5"
    printf '{"state":"%s","merged":%s,"ci_conclusion":%s,"review_blockers_count":%s,"timed_out":%s}\n' \
        "$state" "$merged" "$ci_conclusion" "$review_blockers" "$timed_out"
}

deadline=$(( $(date +%s) + TIMEOUT ))
# Last-known values, emitted if the loop times out before a fresh poll completes.
ci_conclusion="null"
review_blockers=0

while true; do
    if [[ "$(date +%s)" -ge "$deadline" ]]; then
        emit "timeout" "false" "$ci_conclusion" "$review_blockers" "true"
        exit 0
    fi

    # One fetch yields state, merged, and the head SHA used for the CI lookup.
    pr_fields=$(gh api "repos/${REPO}/pulls/${PR_NUMBER}" \
        --jq '"\(.state)\t\(.merged)\t\(.head.sha)"' 2>/dev/null) || {
        echo "Warning: gh api call failed; retrying in ${POLL_INTERVAL}s" >&2
        sleep "$POLL_INTERVAL"
        continue
    }
    IFS=$'\t' read -r pr_state pr_merged head_sha <<< "$pr_fields"

    # Terminal: merged.
    if [[ "$pr_merged" == "true" ]]; then
        emit "merged" "true" '"success"' "0" "false"
        exit 0
    fi

    # Terminal: closed without merge.
    if [[ "$pr_state" == "closed" ]]; then
        emit "closed" "false" "null" "0" "false"
        exit 0
    fi

    # CI status on the head SHA: any failure wins; success only when nothing pends.
    ci_conclusion="null"
    if [[ -n "$head_sha" ]]; then
        checks_json=$(gh api "repos/${REPO}/commits/${head_sha}/check-runs" \
            --jq '[.check_runs[] | select(.conclusion != null) | .conclusion]' 2>/dev/null) || checks_json="[]"

        if printf '%s' "$checks_json" | grep -q '"failure"\|"timed_out"\|"startup_failure"'; then
            emit "ci_failed" "false" '"failure"' "0" "false"
            exit 0
        elif printf '%s' "$checks_json" | grep -q '"success"'; then
            pending=$(gh api "repos/${REPO}/commits/${head_sha}/check-runs" \
                --jq '[.check_runs[] | select(.status == "in_progress" or .status == "queued")] | length' 2>/dev/null) || pending=0
            if [[ "$pending" == "0" ]]; then
                ci_conclusion='"success"'
            else
                ci_conclusion='"pending"'
            fi
        elif printf '%s' "$checks_json" | grep -q '.'; then
            ci_conclusion='"pending"'
        fi
    fi

    # Unresolved review threads (the accurate "blocking review" signal): catches
    # CodeRabbit's COMMENTED + inline reviews, which a CHANGES_REQUESTED-state
    # count misses, and ignores stale never-dismissed reviews.
    review_blockers=$(gh api graphql -f query='
        query($owner:String!, $repo:String!, $pr:Int!) {
          repository(owner:$owner, name:$repo) {
            pullRequest(number:$pr) {
              reviewThreads(first:100) { nodes { isResolved } }
            }
          }
        }' -F owner="$OWNER" -F repo="$REPO_NAME" -F pr="$PR_NUMBER" \
        --jq '[.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved == false)] | length' 2>/dev/null) || review_blockers=0

    if [[ "$review_blockers" -gt 0 ]]; then
        emit "review_blocked" "false" "$ci_conclusion" "$review_blockers" "false"
        exit 0
    fi

    remaining=$(( deadline - $(date +%s) ))
    if [[ "$remaining" -le 0 ]]; then
        continue  # deadline passed mid-poll; top-of-loop check emits timeout
    elif [[ "$remaining" -lt "$POLL_INTERVAL" ]]; then
        sleep "$remaining"
    else
        sleep "$POLL_INTERVAL"
    fi
done
