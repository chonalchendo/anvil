#!/bin/sh
# Fake `claude` for happy-path adapter tests. Reads the prompt from stdin (we
# discard it), prints a canonical stream-json transcript to stdout, exits 0.
cat >/dev/null
cat <<'JSON'
{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Working on it."}]}}
{"type":"result","subtype":"success","duration_ms":1234,"duration_api_ms":1100,"total_cost_usd":0.0042,"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":7,"cache_read_input_tokens":3},"is_error":false,"result":"Done."}
JSON
