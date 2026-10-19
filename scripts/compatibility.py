#!/usr/bin/env python3
"""Compare two binaries on the same private 20+20 history manifest.

The manifest is a JSON array of {"family": "codex"|"claude-code", "path": ...}.
Keep it outside the repository. Reports contain only aggregate categories, line
numbers, and a digest of the corpus, never paths, source text, or raw diagnostics.
"""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile


def classify(result):
    if result.returncode == 0:
        return "imported", "imported", None
    error = result.stderr.decode("utf-8", errors="replace")
    line = re.search(r"\bline (\d+):", error)
    line = int(line.group(1)) if line else None
    for phrase, category in (
        ("unknown trace format", "unknown"),
        ("prefix limit reached", "prefix exhausted"),
        ("ambiguous trace format", "ambiguous"),
        ("read trace prefix", "prefix read error"),
    ):
        if phrase in error:
            return "detection", category, line
    for phrase, category in (
        ('unsupported event "world_state"', "world_state"),
        ('unsupported event "response_item"/"compaction"', "compaction"),
        ("unsupported event last-prompt", "last-prompt"),
        ("unsupported event attachment", "attachment"),
        ("unsupported event mode", "mode"),
        ("unsupported event permission-mode", "permission-mode"),
        ("unsupported event atis-latch", "atis-latch"),
        ('unsupported lifecycle event "thread_settings_applied"', "thread_settings_applied"),
        ('unsupported lifecycle event "item_completed"', "item_completed"),
        ("unsupported lifecycle event", "unsupported lifecycle"),
        ("unsupported tool event", "unsupported tool"),
        ("unsupported acquisition", "unsupported acquisition"),
        ("unsupported event", "other unsupported event"),
        ("unsupported attachment variant", "unsupported attachment variant"),
        ("unsupported attachment shape", "unsupported attachment shape"),
        ("invalid SessionStart hook", "invalid SessionStart hook"),
        ("message before task", "message before task"),
        ("missing session ID", "missing session ID"),
        ("unsupported content block", "unsupported content block"),
        ("unsupported user text block", "unsupported user text block"),
        ("last-prompt", "navigation limitation"),
        ("multiple turns", "multiple turns"),
        ("expected one nonempty task", "multiple or invalid tasks"),
        ("invalid event", "malformed event"),
        ("trace does not contain a task", "no supported task"),
    ):
        if phrase in error:
            return "adapter", category, line
    return "adapter", "other validation failure", line


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--before", required=True, type=Path)
    parser.add_argument("--after", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    entries = json.loads(args.manifest.read_text())
    if Counter(entry["family"] for entry in entries) != {"codex": 20, "claude-code": 20}:
        parser.error("manifest must select exactly 20 histories per family")
    if len({str(Path(entry["path"]).resolve()) for entry in entries}) != 40:
        parser.error("manifest must select 40 distinct histories")
    results = {"before": Counter(), "after": Counter()}
    digest = hashlib.sha256()
    with tempfile.TemporaryDirectory(prefix="unfriction-compat-") as directory:
        for i, entry in enumerate(entries):
            data = Path(entry["path"]).read_bytes()
            digest.update(entry["family"].encode() + b"\0" + hashlib.sha256(data).digest())
            snapshot = Path(directory) / f"history-{i}.jsonl"
            snapshot.write_bytes(data)
            for label, binary in (("before", args.before), ("after", args.after)):
                try:
                    result = subprocess.run([str(binary.resolve()), "inspect", str(snapshot)],
                                            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
                                            timeout=30, check=False)
                    stage, category, line = classify(result)
                except subprocess.TimeoutExpired:
                    stage, category, line = "execution", "timeout", None
                results[label][(entry["family"], stage, category, line)] += 1
    report = {"histories_per_family": 20, "corpus_sha256": digest.hexdigest()}
    for label, counts in results.items():
        report[label] = [{"family": family, "stage": stage, "category": category,
                          "line": line, "count": count}
                         for (family, stage, category, line), count in counts.items()]
    args.output.write_text(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError):
        raise SystemExit("Compatibility experiment failed; check manifest and binary access. Private details suppressed.") from None
