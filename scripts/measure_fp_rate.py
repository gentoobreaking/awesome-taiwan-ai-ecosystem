#!/usr/bin/env python3
"""Measure the MCP false positive rate on a v2 entity export.

Usage:
    python3 scripts/measure_fp_rate.py registry/taiwan-ai-ecosystem.json
    python3 scripts/measure_fp_rate.py --db data/registry.db

Reads the v2 entity export (or the database) and computes the FP rate
against the ground truth fixtures in tests/fixtures/ground_truth/.
Writes the report to scripts/fp_rate_report.md.

Spec §58: FP rate must be below 5% (PASS) or 2% (EXCELLENT).
"""

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parent.parent
FIXTURE_ROOT = REPO_ROOT / "tests" / "fixtures" / "ground_truth"
REPORT_PATH = Path(__file__).resolve().parent / "fp_rate_report.md"


def load_ground_truth() -> list[dict[str, Any]]:
    fixtures = []
    for label in ("positive", "negative"):
        d = FIXTURE_ROOT / label
        if not d.is_dir():
            continue
        for f in sorted(d.glob("*.json")):
            fixtures.append(json.loads(f.read_text()))
    return fixtures


def load_entities(source: str) -> list[dict[str, Any]]:
    p = Path(source)
    if not p.exists():
        sys.exit(f"Entity export not found: {p}")
    data = json.loads(p.read_text())
    if isinstance(data, dict) and "entities" in data:
        return data["entities"]
    if isinstance(data, list):
        return data
    sys.exit(f"Unsupported export shape in {p}")


def classification_is_mcp_server(entity: dict[str, Any]) -> bool:
    cls = entity.get("classification") or {}
    primary = cls.get("primary") or ""
    return primary == "MCP_SERVER"


def measure(entities: list[dict[str, Any]]) -> dict[str, Any]:
    fixtures = load_ground_truth()
    tp = fp = tn = fn = 0
    misclassified: list[dict[str, Any]] = []
    matched = 0
    by_id = {e.get("id"): e for e in entities}

    for f in fixtures:
        eid = f.get("id")
        entity = by_id.get(eid)
        if entity is None:
            # Ground truth fixture that hasn't been classified yet
            # (e.g. when running against a partial export).
            continue
        matched += 1
        predicted = classification_is_mcp_server(entity)
        is_mcp = f.get("is_mcp_server", False)
        if is_mcp and predicted:
            tp += 1
        elif is_mcp and not predicted:
            fn += 1
            misclassified.append({"id": eid, "name": f.get("name"), "expected": True, "predicted": False})
        elif not is_mcp and predicted:
            fp += 1
            misclassified.append({"id": eid, "name": f.get("name"), "expected": False, "predicted": True})
        else:
            tn += 1

    total_positive = tp + fn
    total_negative = tn + fp
    precision = tp / (tp + fp) if (tp + fp) > 0 else 0.0
    recall = tp / total_positive if total_positive > 0 else 0.0
    fpr = fp / (tp + fp) if (tp + fp) > 0 else 0.0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0.0
    if fpr < 0.02:
        status = "EXCELLENT"
    elif fpr < 0.05:
        status = "PASS"
    else:
        status = "FAIL"

    return {
        "matched": matched,
        "total_fixtures": len(fixtures),
        "tp": tp, "fp": fp, "tn": tn, "fn": fn,
        "precision": precision,
        "recall": recall,
        "fpr": fpr,
        "f1": f1,
        "status": status,
        "misclassified": misclassified,
    }


def write_report(report: dict[str, Any], source: str) -> None:
    md = []
    md.append("# FP Rate Report (T107)\n")
    md.append(f"Source: `{source}`\n")
    md.append(f"Total fixtures: **{report['total_fixtures']}** ({report['matched']} matched in export)\n")
    md.append("\n## Counts\n")
    md.append(f"- True Positive: {report['tp']}")
    md.append(f"- False Positive: {report['fp']}")
    md.append(f"- True Negative: {report['tn']}")
    md.append(f"- False Negative: {report['fn']}\n")
    md.append("## Metrics\n")
    md.append(f"- Precision: {report['precision']:.4f}")
    md.append(f"- Recall: {report['recall']:.4f}")
    md.append(f"- F1: {report['f1']:.4f}")
    md.append(f"- **FP Rate: {report['fpr']:.4f}**\n")
    md.append(f"**Status: {report['status']}**\n")
    if report["misclassified"]:
        md.append("## Misclassified\n")
        for m in report["misclassified"]:
            md.append(f"- `{m['id']}` {m['name']} — expected_mcp={m['expected']} predicted_mcp={m['predicted']}")
    REPORT_PATH.write_text("\n".join(md) + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "source",
        nargs="?",
        default="registry/taiwan-ai-ecosystem.json",
        help="Path to a v2 entity export (JSON file with 'entities' array or a bare array)",
    )
    args = parser.parse_args()

    entities = load_entities(args.source)
    report = measure(entities)
    write_report(report, args.source)

    print(f"FP rate: {report['fpr']:.4f}  Status: {report['status']}")
    print(f"Report: {REPORT_PATH}")
    if report["status"] == "FAIL":
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
