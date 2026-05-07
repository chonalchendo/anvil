#!/bin/sh
cat >/dev/null
echo '{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}'
sleep 30
echo '{"type":"result","subtype":"success","duration_ms":30000,"duration_api_ms":29000,"total_cost_usd":0.01,"usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"is_error":false,"result":"too late"}'
