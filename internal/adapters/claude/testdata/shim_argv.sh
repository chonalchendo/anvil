#!/bin/sh
# Fake `claude` that records its argv (one arg per line) to the file named by
# $ANVIL_SHIM_ARGV_FILE, then emits a canonical success transcript so Run
# completes cleanly. Lets a test assert the adapter's per-phase tool wall
# reaches the spawned process argv.
for a in "$@"; do printf '%s\n' "$a"; done >"$ANVIL_SHIM_ARGV_FILE"
cat >/dev/null
cat <<'JSON'
{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}
{"type":"result","subtype":"success","duration_ms":1,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},"is_error":false,"result":"Done."}
JSON
