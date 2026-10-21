#!/usr/bin/env python3
"""Builds a Markdown coverage report from Go coverage profiles (go test -coverprofile).

Usage: coverage-report.py CURRENT_PROFILE [BASE_PROFILE]

Coverage is statement-based like `go tool cover`, aggregated per package. If a profile of main
is given (BASE_PROFILE), the report shows the difference per package and in total.
"""
import os
import sys
from collections import defaultdict

MODULE = "github.com/dreitier/backmon"
MARKER = "<!-- coverage-report -->"


def parse(path):
    """Returns {package: (covered statements, total statements)} or None if the file doesn't exist."""
    if not path or not os.path.isfile(path):
        return None
    blocks = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            # github.com/dreitier/backmon/config/raw.go:12.34,15.2 3 1
            block, stmts, count = line.rsplit(" ", 2)
            stmts, count = int(stmts), int(count)
            # the same block can appear more than once (e.g. merged profiles): count it once
            prev = blocks.get(block)
            blocks[block] = (stmts, max(count, prev[1] if prev else 0))
    packages = defaultdict(lambda: [0, 0])
    for block, (stmts, count) in blocks.items():
        file = block.rsplit(":", 1)[0]
        package = os.path.dirname(file)
        package = package[len(MODULE):].lstrip("/") or "."
        packages[package][1] += stmts
        if count > 0:
            packages[package][0] += stmts
    return {p: tuple(v) for p, v in packages.items()}


def percent(covered_total):
    covered, total = covered_total
    return 100.0 * covered / total if total else 0.0


def total(packages):
    return (sum(c for c, _ in packages.values()), sum(t for _, t in packages.values()))


def delta(current, base):
    d = current - base
    if abs(d) < 0.05:
        return "±0.0"
    return f"{'▲' if d > 0 else '▼'} {d:+.1f}"


def main():
    if len(sys.argv) < 2:
        sys.exit(__doc__)
    current = parse(sys.argv[1])
    if current is None:
        sys.exit(f"coverage profile {sys.argv[1]} not found")
    base = parse(sys.argv[2]) if len(sys.argv) > 2 else None

    cur_total = percent(total(current))
    lines = [MARKER, "### Test coverage", ""]
    if base:
        base_total = percent(total(base))
        lines.append(f"**{cur_total:.1f}%** of statements covered (main: {base_total:.1f}%, {delta(cur_total, base_total)})")
        lines += ["", "| Package | main | this PR | Δ |", "|---|---:|---:|---:|"]
        for package in sorted(set(current) | set(base)):
            if package not in current:
                lines.append(f"| `{package}` | {percent(base[package]):.1f}% | removed | |")
            elif package not in base:
                lines.append(f"| `{package}` | new | {percent(current[package]):.1f}% | |")
            else:
                c, b = percent(current[package]), percent(base[package])
                lines.append(f"| `{package}` | {b:.1f}% | {c:.1f}% | {delta(c, b)} |")
    else:
        lines.append(f"**{cur_total:.1f}%** of statements covered (no coverage of main available for comparison)")
        lines += ["", "| Package | Coverage |", "|---|---:|"]
        for package in sorted(current):
            lines.append(f"| `{package}` | {percent(current[package]):.1f}% |")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
