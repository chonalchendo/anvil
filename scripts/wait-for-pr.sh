#!/usr/bin/env bash
# Out-of-band PR poller: blocks until a terminal state is reached, then emits
# one JSON result. Invoke once from an agent tool call rather than polling
# in-loop, which burns tokens on every iteration of the LLM context.
#
# Terminal states:
#   merged              — PR was merged
#   closed              — PR closed without merge
#   review_blocked      — blocking review comments exist (CodeRabbit or human)
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
  review_blockers_count number of open blocking review threads
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

# Resolve repo from git remote when not supplied.
if [[ -z "$REPO" ]]; then
    REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null) || {
        echo "Error: could not resolve repo — pass --repo owner/repo" >&2
        exit 1
    }
fi

emit() {
    local state="$1" merged="$2" ci_conclusion="$3" review_blockers="$4" timed_out="$5"
    printf '{"state":"%s","merged":%s,"ci_conclusion":%s,"review_blockers_count":%s,"timed_out":%s}\n' \
        "$state" "$merged" "$ci_conclusion" "$review_blockers" "$timed_out"
}

deadline=$(( $(date +%s) + TIMEOUT ))

while true; do
    now=$(date +%s)

    # Fetch PR state in one call.
    pr_json=$(gh api "repos/${REPO}/pulls/${PR_NUMBER}" \
        --jq '{state: .state, merged: .merged, draft: .draft}' 2>/dev/null) || {
        echo "Warning: gh api call failed; retrying in ${POLL_INTERVAL}s" >&2
        sleep "$POLL_INTERVAL"
        continue
    }

    pr_state=$(printf '%s' "$pr_json" | grep '"state"' | grep -oE '"[^"]+"' | tail -1 | tr -d '"')
    pr_merged=$(printf '%s' "$pr_json" | grep '"merged"' | grep -oE '(true|false)' | head -1)

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

    # Check CI status on the PR's head SHA.
    head_sha=$(gh api "repos/${REPO}/pulls/${PR_NUMBER}" --jq '.head.sha' 2>/dev/null) || true

    ci_conclusion="null"
    if [[ -n "$head_sha" ]]; then
        # Combine check-runs and commit statuses; pick the worst conclusion.
        checks_json=$(gh api "repos/${REPO}/commits/${head_sha}/check-runs" \
            --jq '[.check_runs[] | select(.conclusion != null) | .conclusion]' 2>/dev/null) || checks_json="[]"

        if printf '%s' "$checks_json" | grep -q '"failure"\|"timed_out"\|"startup_failure"'; then
            ci_conclusion='"failure"'
            emit "ci_failed" "false" "$ci_conclusion" "0" "false"
            exit 0
        elif printf '%s' "$checks_json" | grep -q '"success"'; then
            # Only mark success if no runs are still pending.
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

    # Check for blocking review comments (unresolved threads with changes-requested or
    # inline comments from active reviewers — use the review list for changes_requested).
    review_blockers=0
    reviews_json=$(gh api "repos/${REPO}/pulls/${PR_NUMBER}/reviews" \
        --jq '[.[] | select(.state == "CHANGES_REQUESTED")] | length' 2>/dev/null) || reviews_json=0
    review_blockers="$reviews_json"

    if [[ "$review_blockers" -gt 0 ]]; then
        emit "review_blocked" "false" "$ci_conclusion" "$review_blockers" "false"
        exit 0
    fi

    # Check timeout before sleeping.
    if [[ "$now" -ge "$deadline" ]]; then
        emit "timeout" "false" "$ci_conclusion" "$review_blockers" "true"
        exit 0
    fi

    remaining=$(( deadline - now ))
    if [[ "$remaining" -lt "$POLL_INTERVAL" ]]; then
        sleep "$remaining"
    else
        sleep "$POLL_INTERVAL"
    fi

    # Final timeout check after sleep.
    if [[ $(date +%s) -ge "$deadline" ]]; then
        emit "timeout" "false" "$ci_conclusion" "$review_blockers" "true"
        exit 0
    fi
done
