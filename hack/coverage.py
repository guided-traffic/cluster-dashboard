#!/usr/bin/env python3
"""Go coverage, merged and summed: the numbers of the CI coverage report and of make.

  hack/coverage.py total PROFILE...
      prints the statement coverage of the profiles merged, "total: 87.3%"
  hack/coverage.py report OUTDIR PROFILE...
      writes OUTDIR/merged.out, OUTDIR/summary.md (one row per package),
      OUTDIR/total.txt (the percentage) and OUTDIR/badge.json (shields.io)

The profiles may come from different Go modules (the backend and the frontend's
file server) and from different runs (unit and integration): a block that
several runs measured appears in each, so blocks are merged by their location,
and a block counts as covered when any run covered it. No source file is read,
so no module has to resolve the other's packages.
"""
import collections
import json
import os
import sys


def read_blocks(paths):
    blocks = {}
    for path in paths:
        with open(path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line or line.startswith("mode:"):
                    continue
                loc, statements, count = line.rsplit(" ", 2)
                statements, count = int(statements), int(count)
                prev = blocks.get(loc)
                if prev is None or count > prev[1]:
                    blocks[loc] = (statements, count)
    return blocks


def percent(covered, total):
    return 100.0 * covered / total if total else 0.0


def totals(blocks):
    total = sum(s for s, _ in blocks.values())
    covered = sum(s for s, c in blocks.values() if c > 0)
    return covered, total


def per_package(blocks):
    packages = collections.defaultdict(lambda: [0, 0])
    for loc, (statements, count) in blocks.items():
        package = loc.rsplit(":", 1)[0].rsplit("/", 1)[0]
        packages[package][0] += statements
        if count > 0:
            packages[package][1] += statements
    return packages


def color(pct):
    for bound, name in ((90, "brightgreen"), (80, "green"), (70, "yellow"), (60, "orange")):
        if pct >= bound:
            return name
    return "red"


def main(argv):
    if len(argv) < 3 or argv[1] not in ("total", "report"):
        print(__doc__, file=sys.stderr)
        return 2
    if argv[1] == "total":
        covered, total = totals(read_blocks(argv[2:]))
        print(f"total: {percent(covered, total):.1f}% of {total} statements")
        return 0
    outdir, profiles = argv[2], argv[3:]
    if not profiles:
        print("no coverage profile to report: run make test-unit-coverage or make test-integration-coverage", file=sys.stderr)
        return 1
    blocks = read_blocks(profiles)
    os.makedirs(outdir, exist_ok=True)
    with open(os.path.join(outdir, "merged.out"), "w", encoding="utf-8") as fh:
        fh.write("mode: set\n")
        for loc in sorted(blocks):
            statements, count = blocks[loc]
            fh.write(f"{loc} {statements} {1 if count > 0 else 0}\n")
    rows = ["| Package | Coverage | Statements |", "|---|---:|---:|"]
    for package, (total, covered) in sorted(per_package(blocks).items()):
        rows.append(f"| `{package}` | {percent(covered, total):.1f}% | {covered}/{total} |")
    with open(os.path.join(outdir, "summary.md"), "w", encoding="utf-8") as fh:
        fh.write("\n".join(rows) + "\n")
    covered, total = totals(blocks)
    pct = round(percent(covered, total), 1)
    with open(os.path.join(outdir, "total.txt"), "w", encoding="utf-8") as fh:
        fh.write(f"{pct}\n")
    with open(os.path.join(outdir, "badge.json"), "w", encoding="utf-8") as fh:
        json.dump({"schemaVersion": 1, "label": "coverage", "message": f"{pct}%", "color": color(pct)}, fh)
        fh.write("\n")
    print(f"coverage of {', '.join(profiles)}: {pct}% of {total} statements")
    print("\n".join(rows))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
