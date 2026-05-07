#!/bin/sh
cat >/dev/null
cat <<'JSON'
{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}
{"type":"result","subtype":"error_during_execution","duration_ms":250,"duration_api_ms":200,"total_cost_usd":0.0001,"usage":{"input_tokens":10,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"is_error":true,"result":"Tool failed."}
JSON
exit 1
