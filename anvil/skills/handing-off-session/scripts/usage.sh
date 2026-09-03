#!/usr/bin/env bash
# usage.sh — report token usage per model and per anvil agent type from local
# Claude Code transcripts.
#
# Transcripts log one JSONL line per content block, so a naive sum over
# `message.usage` overcounts roughly twofold: dedupe by `message.id` first.
# Subagent transcripts sit under <project>/<session>/subagents/*.jsonl with
# no agent-type field, so type is recovered from the first user message in
# that file (the dispatch prompt).
#
# Usage:
#   usage.sh --since <N>d [--dir <projects-dir>]
#
# --dir defaults to ~/.claude/projects. --since is required and only
# supports the "<N>d" shape (days).
#
# Output: one JSON array on stdout, one object per (model, agent_type) pair:
#   {"model":"...","agent_type":"...","messages":N,"input":N,
#    "cache_create":N,"cache_read":N,"output":N}
#
# Depends only on bash, jq, and find.

set -euo pipefail

dir="$HOME/.claude/projects"
since=""

while [ $# -gt 0 ]; do
    case "$1" in
    --dir)
        dir="$2"
        shift 2
        ;;
    --since)
        since="$2"
        shift 2
        ;;
    *)
        echo "usage.sh: unknown argument: $1" >&2
        exit 2
        ;;
    esac
done

if [ -z "$since" ]; then
    echo "usage.sh: --since <N>d is required" >&2
    exit 2
fi

days="${since%d}"
case "$days" in
'' | *[!0-9]*)
    echo "usage.sh: --since must look like <N>d, got: $since" >&2
    exit 2
    ;;
esac

now=$(date -u +%s)
cutoff=$((now - days * 86400))

if [ ! -d "$dir" ]; then
    echo "[]"
    exit 0
fi

# agent_type for a subagent transcript is recovered from the first user
# message in the file: the dispatch prompt names the role.
classify_subagent() {
    local file=$1
    local prompt
    prompt=$(jq -r 'select(.type == "user") | .message.content
        | if type == "array" then map(select(.type == "text") | .text) | join(" ") else tostring end' "$file" 2>/dev/null | head -1)
    case "$prompt" in
    *"Complete anvil issue"*) echo "worker" ;;
    *"Review PR"* | *"PR: #"*) echo "reviewer" ;;
    *"findings"*) echo "responder" ;;
    *) echo "subagent-unknown" ;;
    esac
}

rows=$(mktemp)
trap 'rm -f "$rows"' EXIT

while IFS= read -r -d '' file; do
    case "$file" in
    */subagents/*) agent_type=$(classify_subagent "$file") ;;
    *) agent_type="main" ;;
    esac
    jq -c --arg agent_type "$agent_type" --argjson cutoff "$cutoff" '
        select(.type == "assistant")
        | select(.timestamp != null)
        | select((.timestamp | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601) >= $cutoff)
        | {
            id: .message.id,
            model: .message.model,
            agent_type: $agent_type,
            input: (.message.usage.input_tokens // 0),
            cache_create: (.message.usage.cache_creation_input_tokens // 0),
            cache_read: (.message.usage.cache_read_input_tokens // 0),
            output: (.message.usage.output_tokens // 0)
        }
    ' "$file" 2>/dev/null >>"$rows" || true
done < <(find "$dir" -type f -name '*.jsonl' -print0)

jq -s '
    unique_by(.id)
    | group_by([.model, .agent_type])
    | map({
        model: .[0].model,
        agent_type: .[0].agent_type,
        messages: length,
        input: (map(.input) | add),
        cache_create: (map(.cache_create) | add),
        cache_read: (map(.cache_read) | add),
        output: (map(.output) | add)
    })
' "$rows"
