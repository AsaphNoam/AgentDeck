# Transcript usage audit

`audit.py` scans Claude Code and Codex JSONL transcripts and reports the raw token counters recorded by each event. It filters on the event timestamp (inclusive dates), never filesystem modification time, and deduplicates Claude records sharing a `requestId` within each file/session. Claude sidechains and Codex subagents/guardians are separate when transcript metadata identifies them. Codex rollout files are represented once: the final cumulative `total_token_usage` on or before `--to` is reduced by the latest strictly pre-`--from` snapshot, so a long-lived session contributes only its interval delta. Repeated snapshots are not added together, and counter resets never produce negative values. Missing metadata remains classified as main.

The JSON report contains provider/class totals, provider-wide main-plus-child totals, per-session totals, and every retained request record, making before/after workflow comparisons reproducible. It does not estimate subscription quotas or convert counters into quota usage.
Counter names retain each provider's meaning and may overlap—for example, Codex cached input is also
part of its input total—so do not add unlike columns into a synthetic total.

```sh
scripts/transcript-usage/audit.py --from 2026-08-01 --to 2026-08-31 \
  --claude-root ~/.claude/projects --codex-root ~/.codex/sessions --json

# Compare one AgentDeck project across both providers
scripts/transcript-usage/audit.py --cwd /Users/mcnoam/Projects/AgentDeck \
  --from 2026-08-01 --to 2026-08-31 --json
```
