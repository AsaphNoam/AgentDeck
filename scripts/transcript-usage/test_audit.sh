#!/bin/sh
set -eu
test_root=$(mktemp -d "${TMPDIR:-/tmp}/transcript-usage.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
mkdir -p "$test_root/claude" "$test_root/codex"
cat > "$test_root/claude/a.jsonl" <<'EOF'
{"timestamp":"2026-08-01T23:59:00Z","cwd":"/workspace/agentdeck","requestId":"same","message":{"usage":{"input_tokens":10,"output_tokens":2}}}
{"timestamp":"2026-08-01T23:59:01Z","cwd":"/workspace/agentdeck","requestId":"same","message":{"usage":{"input_tokens":10,"output_tokens":2}}}
{"timestamp":"2026-08-02T00:00:00Z","cwd":"/workspace/agentdeck","requestId":"side","isSidechain":true,"usage":{"input_tokens":3,"output_tokens":1}}
{"timestamp":"2026-08-02T00:00:01Z","cwd":"/workspace/other","requestId":"excluded","usage":{"input_tokens":99}}
{"timestamp":"2026-07-31T23:59:59Z","requestId":"old","usage":{"input_tokens":99}}
EOF
cat > "$test_root/codex/c.jsonl" <<'EOF'
null
[]
{"timestamp":"2026-08-02T11:59:59Z","type":"session_meta","payload":null}
{"timestamp":"2026-08-02T12:00:00Z","type":"session_meta","payload":{"id":"codex-sub","cwd":"/workspace/agentdeck","source":{"subagent":{"thread_spawn":{"parent_thread_id":"p"}}},"thread_source":"subagent"}}
{"timestamp":"2026-07-31T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":40,"cached_input_tokens":20}}}}
{"timestamp":"2026-08-02T12:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":7,"output_tokens":4,"cached_input_tokens":2}}}}
{"timestamp":"2026-08-02T12:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":112,"output_tokens":46,"cached_input_tokens":23}}}}
EOF
cat > "$test_root/codex/g.jsonl" <<'EOF'
{"timestamp":"2026-08-02T12:00:00Z","type":"session_meta","payload":{"id":"codex-guardian","cwd":"/workspace/agentdeck","source":{"subagent":{"other":"guardian"}}}}
{"timestamp":"2026-08-02T12:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5,"output_tokens":1}}}}
EOF
out=$(python3 scripts/transcript-usage/audit.py --claude-root "$test_root/claude" --codex-root "$test_root/codex" --cwd /workspace/agentdeck --from 2026-08-01 --to 2026-08-02 --json 2>"$test_root/stderr")
grep -q 'skipped non-object Codex record' "$test_root/stderr"
grep -q 'skipped Codex session metadata with non-object payload' "$test_root/stderr"
printf '%s' "$out" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["totals"]["claude/main"]["input_tokens"] == 10; assert x["totals"]["claude/sidechain/subagent"]["input_tokens"] == 3; assert x["totals"]["codex/subagent"]["input_tokens"] == 12; assert x["totals"]["codex/guardian"]["input_tokens"] == 5; assert x["provider_totals_main_plus_child"]["claude"]["input_tokens"] == 13; assert x["provider_totals_main_plus_child"]["codex"]["input_tokens"] == 17; assert len(x["requests"]) == 4; assert x["quota_inference"] is False'
echo "transcript usage audit fixture: ok"
