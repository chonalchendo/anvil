#!/bin/sh
cat >/dev/null
cat <<'JSON'
{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}
{"type":"result","subtype":"error_during_execution","duration_ms":80,"duration_api_ms":75,"total_cost_usd":0,"usage":{"input_tokens":5,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"is_error":true,"result":"Claude AI usage limit reached. Try again at 5pm UTC."}
JSON
exit 1
