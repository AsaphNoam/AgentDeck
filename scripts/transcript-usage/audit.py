#!/usr/bin/env python3
"""Report raw token counters from Claude Code and Codex JSONL transcripts."""

import argparse
import datetime as dt
import json
import os
from collections import defaultdict
from pathlib import Path

COUNTERS = (
    "input_tokens",
    "output_tokens",
    "cache_creation_input_tokens",
    "cache_read_input_tokens",
    "cached_input_tokens",
    "cache_write_input_tokens",
    "reasoning_tokens",
    "reasoning_output_tokens",
    "total_tokens",
)
TIME_KEYS = ("timestamp", "created_at", "createdAt", "event_time", "time")


def event_date(value):
    if not isinstance(value, str):
        return None
    text = value.strip()
    if len(text) >= 10:
        try:
            return dt.date.fromisoformat(text[:10])
        except ValueError:
            pass
    try:
        return dt.datetime.fromisoformat(text.replace("Z", "+00:00")).date()
    except ValueError:
        return None


def first_value(obj, keys):
    if isinstance(obj, dict):
        for key in keys:
            if key in obj and isinstance(obj[key], (str, int, float)):
                return obj[key]
        for value in obj.values():
            found = first_value(value, keys)
            if found is not None:
                return found
    elif isinstance(obj, list):
        for value in obj:
            found = first_value(value, keys)
            if found is not None:
                return found
    return None


def values_for_keys(obj, keys):
    if isinstance(obj, dict):
        for key, value in obj.items():
            if key in keys and isinstance(value, (str, int, float, bool)):
                yield value
            yield from values_for_keys(value, keys)
    elif isinstance(obj, list):
        for value in obj:
            yield from values_for_keys(value, keys)


def usage_object(obj):
    if isinstance(obj, dict):
        if any(key in obj for key in COUNTERS):
            return obj
        for value in obj.values():
            found = usage_object(value)
            if found is not None:
                return found
    elif isinstance(obj, list):
        for value in obj:
            found = usage_object(value)
            if found is not None:
                return found
    return None


def truthy(obj, names):
    if isinstance(obj, dict):
        if any(bool(obj.get(name)) for name in names):
            return True
        return any(truthy(value, names) for value in obj.values())
    if isinstance(obj, list):
        return any(truthy(value, names) for value in obj)
    return False


def classify(provider, record):
    if provider == "claude":
        if truthy(record, ("isSidechain", "is_sidechain", "sidechain")):
            return "sidechain/subagent"
        return "main"
    if truthy(record, ("guardian", "is_guardian")):
        return "guardian"
    if truthy(record, ("is_subagent", "isSubagent", "subagent", "sidechain")):
        return "subagent"
    for key in ("agent_role", "agentRole", "agent_type", "agentType", "role", "source", "kind"):
        for value in values_for_keys(record, (key,)):
            if isinstance(value, str):
                lowered = value.lower()
                if "guardian" in lowered:
                    return "guardian"
                if "subagent" in lowered or "sidechain" in lowered:
                    return "subagent"
    return "main"


def request_id(record):
    return first_value(record, ("requestId", "request_id"))


def normalized_path(value):
    return os.path.realpath(os.path.expanduser(str(value)))


def codex_class(meta):
    source = meta.get("source", {}) if isinstance(meta, dict) else {}
    text = json.dumps({"source": source, "thread_source": meta.get("thread_source", "")}, ensure_ascii=False).lower()
    if "guardian" in text:
        return "guardian"
    if "subagent" in text or meta.get("thread_source") == "subagent":
        return "subagent"
    return "main"


def scan(root, provider, start, end, cwd=None):
    rows, seen = [], set()
    root = Path(os.path.expanduser(root))
    if not root.exists():
        return rows
    for path in sorted(root.rglob("*.jsonl")):
        try:
            handle = path.open(encoding="utf-8", errors="replace")
        except OSError:
            continue
        with handle:
            for line_no, line in enumerate(handle, 1):
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if cwd is not None:
                    record_cwd = first_value(record, ("cwd",))
                    if record_cwd is None or normalized_path(record_cwd) != cwd:
                        continue
                value = first_value(record, TIME_KEYS)
                date = event_date(str(value)) if value is not None else None
                if date is None or not (start <= date <= end):
                    continue
                usage = usage_object(record)
                if usage is None:
                    continue
                rid = request_id(record)
                if provider == "claude" and rid is not None:
                    key = (str(path), str(rid))
                    if key in seen:
                        continue
                    seen.add(key)
                counters = {key: int(usage.get(key, 0) or 0) for key in COUNTERS if usage.get(key) is not None}
                if not counters:
                    continue
                rows.append({"provider": provider, "class": classify(provider, record),
                             "session": str(path.relative_to(root)), "request": str(rid) if rid is not None else "",
                             "date": date.isoformat(), "line": line_no, "tokens": counters})
    return rows


def scan_codex(root, start, end, cwd=None):
    """Read one per-file delta between cumulative total_token_usage snapshots."""
    rows = []
    root = Path(os.path.expanduser(root))
    if not root.exists():
        return rows
    for path in sorted(root.rglob("*.jsonl")):
        meta, baseline, final = {}, None, None
        try:
            handle = path.open(encoding="utf-8", errors="replace")
        except OSError:
            continue
        with handle:
            for line_no, line in enumerate(handle, 1):
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if record.get("type") == "session_meta":
                    meta = record.get("payload", {})
                    if cwd is not None and normalized_path(meta.get("cwd", "")) != cwd:
                        meta["_excluded"] = True
                payload = record.get("payload")
                if record.get("type") != "event_msg" or not isinstance(payload, dict) or payload.get("type") != "token_count":
                    continue
                date = event_date(str(record.get("timestamp", "")))
                info = payload.get("info")
                usage = info.get("total_token_usage") if isinstance(info, dict) else None
                if date is None or not isinstance(usage, dict):
                    continue
                if date < start:
                    baseline = usage
                elif date <= end:
                    final = (line_no, date, usage)
        if final is None:
            continue
        if meta.get("_excluded"):
            continue
        line_no, date, usage = final
        baseline = baseline or {}
        counters = {}
        for key in COUNTERS:
            if usage.get(key) is None:
                continue
            delta = int(usage[key] or 0) - int(baseline.get(key, 0) or 0)
            # A provider reset can make a cumulative counter decrease; do not
            # turn that reset into a negative usage value.
            counters[key] = max(0, delta)
        if counters:
            rows.append({"provider": "codex", "class": codex_class(meta),
                         "session": str(path.relative_to(root)), "request": str(meta.get("id", "")),
                         "date": date.isoformat(), "line": line_no, "tokens": counters})
    return rows


def report(rows, start, end):
    totals = defaultdict(lambda: defaultdict(int))
    provider_totals = defaultdict(lambda: defaultdict(int))
    sessions = defaultdict(lambda: defaultdict(lambda: defaultdict(int)))
    requests = []
    for row in rows:
        bucket = totals[(row["provider"], row["class"])]
        for key, value in row["tokens"].items():
            bucket[key] += value
        bucket["records"] += 1
        provider_bucket = provider_totals[row["provider"]]
        for key, value in row["tokens"].items():
            provider_bucket[key] += value
        provider_bucket["records"] += 1
        session = sessions[(row["provider"], row["class"])][row["session"]]
        for key, value in row["tokens"].items():
            session[key] += value
        session["records"] += 1
        requests.append(row)
    return {"from": start.isoformat(), "to": end.isoformat(), "event_date_filter": "inclusive",
            "quota_inference": False, "totals": {f"{p}/{c}": dict(v) for (p, c), v in sorted(totals.items())},
            "provider_totals_main_plus_child": {p: dict(v) for p, v in sorted(provider_totals.items())},
            "sessions": {f"{p}/{c}": {s: dict(v) for s, v in sorted(group.items())} for (p, c), group in sorted(sessions.items())},
            "requests": requests}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--claude-root", default="~/.claude/projects")
    parser.add_argument("--codex-root", default="~/.codex/sessions")
    parser.add_argument("--from", dest="start", required=True, help="inclusive YYYY-MM-DD")
    parser.add_argument("--to", dest="end", required=True, help="inclusive YYYY-MM-DD")
    parser.add_argument("--cwd", help="only include records/sessions for this project directory")
    parser.add_argument("--json", action="store_true", help="emit machine-readable JSON")
    args = parser.parse_args()
    start, end = dt.date.fromisoformat(args.start), dt.date.fromisoformat(args.end)
    if end < start:
        parser.error("--to must be on or after --from")
    cwd = normalized_path(args.cwd) if args.cwd else None
    rows = scan(args.claude_root, "claude", start, end, cwd) + scan_codex(args.codex_root, start, end, cwd)
    result = report(rows, start, end)
    if args.json:
        print(json.dumps(result, indent=2, sort_keys=True))
    else:
        print(f"Transcript usage ({result['from']} through {result['to']}, event dates; raw counters)")
        for name, values in result["totals"].items():
            print(name, " ".join(f"{key}={value}" for key, value in sorted(values.items())))
        for provider, values in result["provider_totals_main_plus_child"].items():
            print(f"{provider}/main+child", " ".join(f"{key}={value}" for key, value in sorted(values.items())))
        print(f"Retained rows: {len(rows)} (Claude requests plus Codex session deltas; quota not inferred)")


if __name__ == "__main__":
    main()
