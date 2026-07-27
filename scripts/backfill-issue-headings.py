#!/usr/bin/env python3
r"""Throwaway backfill script — anvil.0203.161-back-catalogue-issues-fail-the.

Inserts stub required-headings into pre-schema issue bodies so `anvil validate`
stops flagging `issue body missing required heading`. Each inserted stub is
visibly marked `_Backfilled stub_` per the issue's non-goal. Not shipped code,
and not wired into the binary — it is retained as the audit and reproducibility
record for a 51-file mutation to a vault that is only loosely versioned.

`find_pos` is a first-occurrence search over the whole body. It is *not*
`ValidateIssue`'s ordered scan (which advances past each match and searches the
remaining suffix); on a corpus where each required heading appears at most once
the two agree, which is why this was safe here.

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

_MARK = (
    "_Backfilled stub_ — heading inserted to satisfy the required-heading schema; "
    "not authored at creation time."
)
_VERIF = _MARK + " Any checks the author recorded are in the sections above (e.g. `## Acceptance criteria`)."

STUBS = {
    "## Problem": f"## Problem\n\n{_MARK}\n",
    "## Non-goals": f"## Non-goals\n\n{_MARK}\n",
    "## Verification": "## Verification\n",
    "### Direct": f"### Direct (unit/integration)\n{_VERIF}\n",
    "### Indirect": f"### Indirect (live smoke)\n{_VERIF}\n",
    "## Links": f"## Links\n\n{_MARK}\n",
}


def find_pos(body, heading):
    idx = body.find("\n" + heading)
    if idx >= 0:
        return idx + 1
    if body.startswith(heading):
        return 0
    return -1


def backfill(body, missing, path="<body>"):
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
        # No leading "\n": the anchor position already sits after a blank line, so
        # prepending one produced a doubled blank line.
        stub = "\n".join(STUBS[h] for h in group).rstrip("\n")
        if anchor is not None:
            pos = present_pos[anchor]
            # find_pos returns -1 for an absent anchor; body[:-1] would silently
            # splice before the final character instead of failing.
            assert pos >= 0, f"anchor {anchor} not found in {path}"
            body = body[:pos] + stub + "\n\n" + body[pos:]
        else:
            # An empty body is already just the blank line after the frontmatter
            # fence, so it needs one newline, not a fresh blank line.
            head = body.rstrip("\n")
            body = head + ("\n\n" if head else "\n") + stub + "\n"
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

        new_body = backfill(body, missing, path)
        if new_body != body:
            with open(path, "w") as f:
                f.write(frontmatter + new_body)
            changed.append(path)

    print(f"changed {len(changed)} files")


if __name__ == "__main__":
    main()
