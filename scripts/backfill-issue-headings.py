#!/usr/bin/env python3
"""Throwaway backfill script — anvil.0203.161-back-catalogue-issues-fail-the.

Inserts stub required-headings into pre-schema issue bodies so `anvil validate`
stops flagging `issue body missing required heading`. Each inserted stub is
visibly marked `_Backfilled stub_` per the issue's non-goal. Not shipped code —
run once, then discard.

Manifest format: one `<path>\t<comma-separated-missing-headings>` line per
affected file. Derive it from a live vault with:

    anvil validate 2>&1 | awk '
      /^\[constraint_violation\]/ { path="" }
      /  path:/ { path=$2 }
      /missing required heading/ && path { print path }
    ' | sort -u

then pair each path with the specific missing headings from the same
`anvil validate` run before invoking: `python3 backfill-issue-headings.py <manifest>`.
"""
import sys

ORDER = ["## Problem", "## Non-goals", "## Verification", "### Direct", "### Indirect", "## Links"]

STUBS = {
    "## Problem": '## Problem\n\n_Backfilled stub — this issue predates the required-heading schema; no ## Problem section was authored originally._\n',
    "## Non-goals": '## Non-goals\n\n_Backfilled stub — this issue predates the required-heading schema; no ## Non-goals section was authored originally._\n',
    "## Verification": '## Verification\n',
    "### Direct": '### Direct (unit/integration)\n_Backfilled stub — this issue predates the Verification schema; no direct check was authored originally._\n',
    "### Indirect": '### Indirect (live smoke)\n_Backfilled stub — this issue predates the Verification schema; no indirect check was authored originally._\n',
    "## Links": '## Links\n\n_Backfilled stub — no links section was authored originally._\n',
}


def find_pos(body, heading):
    idx = body.find("\n" + heading)
    if idx >= 0:
        return idx + 1
    if body.startswith(heading):
        return 0
    return -1


def backfill(body, missing):
    present_pos = {h: find_pos(body, h) for h in ORDER if h not in missing}

    groups = []
    cur = []
    for h in ORDER:
        if h in missing:
            cur.append(h)
        else:
            if cur:
                groups.append((cur, h))
                cur = []
    if cur:
        groups.append((cur, None))

    for group, anchor in reversed(groups):
        stub = "\n" + "\n".join(STUBS[h] for h in group)
        if anchor is not None:
            pos = present_pos[anchor]
            body = body[:pos] + stub.rstrip("\n") + "\n\n" + body[pos:]
        else:
            body = body.rstrip("\n") + "\n" + stub.rstrip("\n") + "\n"
    return body


def main():
    manifest_path = sys.argv[1]
    with open(manifest_path) as f:
        lines = [l.rstrip("\n") for l in f if l.strip()]

    changed = []
    for line in lines:
        path, cols = line.split("\t")
        missing = set(cols.split(","))
        with open(path) as f:
            content = f.read()

        if content.startswith("---\n"):
            end = content.index("\n---\n", 4) + len("\n---\n")
            frontmatter, body = content[:end], content[end:]
        else:
            frontmatter, body = "", content

        new_body = backfill(body, missing)
        if new_body != body:
            with open(path, "w") as f:
                f.write(frontmatter + new_body)
            changed.append(path)

    print(f"changed {len(changed)} files")


if __name__ == "__main__":
    main()
