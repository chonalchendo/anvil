#!/usr/bin/env python3
r"""Throwaway backfill script — anvil.0212.136-back-catalogue-issues-missing.

Two phases over the same file set, because supplying the first unmasks the
second: `internal/cli/validate.go`'s `validateOne` early-returns on the first
frontmatter-schema failure, so `core.ValidateIssue`'s body-heading checks never
ran while `goal:` was missing. Adding `goal:` alone therefore *appears* to
introduce hundreds of `missing required heading` findings that were latent all
along.

Phase 1 — `goal:` frontmatter. Derived from the issue's own content so
`anvil validate` stops flagging `missing_required` findings naming
`field: goal`. Tiers, in order:

  1. first bullet of `## Acceptance…` (matches `## Acceptance` and
     `## Acceptance criteria` alike) -> marked `(backfilled)`
  2. first entry of the frontmatter `acceptance:` list -> `(backfilled)`
  3. otherwise the issue `title` -> `(backfilled — derived from title)`

Tier 3 deliberately does *not* invert `description` or the first sentence of
`## Problem` into a pseudo-goal: those are problem-shaped, and a claiming
worker orienting on `goal:` would read the defect as the objective. The title
is no more informative but is at least not actively misleading, and its marker
says exactly where it came from.

Phase 2 — required body headings. Inserts the `core.RequiredIssueSections`
headings a body is missing, in the order and blank-line layout
`core.ScaffoldSections` renders (`scaffold_sections` below mirrors it), each
carrying the same visible `_Backfilled stub_` marker the sibling
`backfill-issue-headings.py` writes. The marker asserts only that the
heading was inserted post-hoc — it makes no claim about what the author did or
did not write elsewhere in the file, because the 2026-07-26 review of the
sibling `backfill-issue-headings.py` caught exactly that class of untrue
marker in 33 files.

`missing_sections` mirrors `core.ValidateIssue`'s *ordered* scan (advance past
each match, search the remaining suffix) rather than a first-occurrence search,
so the set it repairs is by construction the set validate reports.

Neither phase is shipped code, and neither is wired into the binary — this
file is retained as the audit and reproducibility record for a mutation to a
vault that is only loosely versioned. Both phases are re-runnable: phase 1
re-derives a `goal:` it previously wrote (recognised by its marker) and leaves
a hand-authored one alone; phase 2 is a no-op once the headings are present.

`insert_goal` places the new `goal:` line at its alphabetical position among
top-level frontmatter keys (matching the vault's existing convention), using
`yaml.safe_dump` on a single key so any value needing YAML quoting (colons,
leading punctuation) round-trips safely instead of corrupting the document.

Usage: python3 backfill-issue-goals.py <manifest>
manifest: one vault issue path per line. Derive one live with:

    anvil validate --json | jq -r '
      .[] | select((.code=="missing_required" and .field=="goal")
                   or (.got|test("missing required heading")))
          | .path' | sort -u
"""
import re
import sys

import yaml

MARK = " (backfilled)"
MARK_TITLE = " (backfilled — derived from title)"
MAX_LEN = 120

# Mirror of core.RequiredIssueSections (internal/core/issue_validate.go).
REQUIRED_SECTIONS = [
    "## Problem",
    "## Non-goals",
    "## Verification",
    "### Direct",
    "### Indirect",
    "## Links",
]

# Byte-identical to backfill-issue-headings.py's marker so the two backfills
# read uniformly in the vault and a later cleanup can grep exactly one string.
SECTION_MARK = (
    "_Backfilled stub_ — heading inserted to satisfy the required-heading schema; "
    "not authored at creation time."
)

# A fenced block lifted verbatim into a YAML scalar leaves an unbalanced ```
# inside the frontmatter. Stripped before any bullet/sentence match so no tier
# can carry one out.
FENCE_RE = re.compile(r"(?s)```.*?```")

# Clause boundary: terminal punctuation followed by whitespace. Used to end an
# over-length goal on a complete clause rather than mid-clause.
#
# `:` and `—` are deliberately absent even though they end a clause: both
# *introduce* the payload ("gains a rule: <the rule>"), so cutting there
# discards exactly the operative part the truncation is supposed to preserve.
CLAUSE_END_RE = re.compile(r"[.!?;,](?=\s)")

# A cut shorter than this is a stub, not a predicate (the shortest clause
# boundary in the corpus lands at 5 chars); such a file keeps the abridged
# fragment instead.
MIN_CLAUSE = 40


def frontmatter_split(content):
    if not content.startswith("---\n"):
        return "", content
    end = content.index("\n---\n", 4) + len("\n---\n")
    return content[:end], content[end:]


def first_bullet(text):
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("- "):
            return line[2:].strip()
    return None


def section(body, pattern):
    # Anchored to line-start so a heading name merely *quoted* in prose
    # (e.g. inside a code span elsewhere in the body) is never mistaken
    # for the real heading.
    m = re.search(r"(?m)^" + pattern + r"$", body)
    if not m:
        return None
    rest = body[m.end() :]
    end = re.search(r"\n#{1,3} ", rest)
    return rest[: end.start()] if end else rest


def derive_goal(fm, body):
    """Return (text, marker). See the tier list in the module docstring."""
    # `\b.*` so `## Acceptance` and `## Acceptance criteria` both match. The
    # exact-string probe this replaced silently missed every bare
    # `## Acceptance`, dropping 12 issues with a clean predicate one line away
    # into the problem-shaped fallback.
    acc_section = section(body, r"## Acceptance\b.*")
    if acc_section:
        bullet = first_bullet(acc_section)
        if bullet:
            return bullet, MARK
    acc_list = fm.get("acceptance")
    if isinstance(acc_list, list) and acc_list:
        return str(acc_list[0]), MARK
    return str(fm.get("title", "")), MARK_TITLE


def clean(text):
    return " ".join(FENCE_RE.sub(" ", text).split()).strip().rstrip(".")


def last_clause_end(text):
    """Index of the last top-level clause boundary in text, or -1.

    Boundaries inside an unclosed bracket are skipped: cutting there strands an
    opening `(` or `[` and reads as damage rather than as an abridgement.
    """
    depth = 0
    cut = -1
    for i, ch in enumerate(text):
        if ch in "([":
            depth += 1
        elif ch in ")]":
            depth = max(0, depth - 1)
        elif depth == 0 and CLAUSE_END_RE.match(text, i):
            cut = i
    return cut


def fit(text, suffix):
    budget = MAX_LEN - len(suffix)
    if len(text) <= budget:
        return text + suffix
    head = text[:budget]
    cut = last_clause_end(head)
    if cut >= MIN_CLAUSE:
        return head[:cut].rstrip() + suffix
    # No usable clause boundary: fall back to a whole-word cut, ellipsis-marked
    # so the fragment is visibly abridged.
    return head[: budget - 1].rsplit(" ", 1)[0] + "…" + suffix


def goal_line(value):
    return yaml.safe_dump({"goal": value}, default_flow_style=False, allow_unicode=True).rstrip("\n")


def strip_goal(frontmatter):
    """Drop an existing top-level `goal:` block (key line plus folded continuations)."""
    out = []
    dropping = False
    for line in frontmatter.splitlines(keepends=True):
        if dropping:
            if line.startswith((" ", "\t")):
                continue
            dropping = False
        if line.startswith("goal:"):
            dropping = True
            continue
        out.append(line)
    return "".join(out)


def insert_goal(frontmatter, line):
    lines = frontmatter.splitlines(keepends=True)
    # lines[0] is the opening "---\n", lines[-1] the closing "---\n"; only
    # top-level (unindented "key:") lines are candidate insertion anchors.
    for i in range(1, len(lines) - 1):
        m = re.match(r"^([A-Za-z_]+):", lines[i])
        if m and m.group(1) > "goal":
            return "".join(lines[:i]) + line + "\n" + "".join(lines[i:])
    return "".join(lines[:-1]) + line + "\n" + lines[-1]


def scaffold_sections(headings):
    """Mirror of core.ScaffoldSections: each heading on its own line, blank line between."""
    return "".join("\n" + h + "\n" for h in headings)


def missing_sections(body):
    """Mirror of core.ValidateIssue's ordered heading scan."""
    pos = 0
    missing = []
    for h in REQUIRED_SECTIONS:
        idx = body.find("\n" + h, pos)
        if idx < 0 and not body[pos:].startswith(h):
            missing.append(h)
            continue
        if idx >= 0:
            pos = idx + len(h) + 1
        else:
            pos += len(h)
    return missing


def find_pos(body, heading):
    idx = body.find("\n" + heading)
    if idx >= 0:
        return idx + 1
    if body.startswith(heading):
        return 0
    return -1


def backfill_sections(body, missing, path="<body>"):
    present_pos = {h: find_pos(body, h) for h in REQUIRED_SECTIONS if h not in missing}

    groups = []
    cur = []
    for h in REQUIRED_SECTIONS:
        if h in missing:
            cur.append(h)
        else:
            if cur:
                groups.append((cur, h))
                cur = []
    if cur:
        groups.append((cur, None))

    for group, anchor in reversed(groups):
        # scaffold_sections' leading "\n" is the blank line *between* blocks; the
        # first one is dropped because the splice point already sits after a
        # blank line and a second produced a doubled blank line.
        stub = "".join(
            scaffold_sections([h]).lstrip("\n") + "\n" + SECTION_MARK + "\n\n" for h in group
        ).rstrip("\n")
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
        paths = [line.strip() for line in f if line.strip()]

    goals = 0
    sections = 0
    for path in paths:
        with open(path) as f:
            content = f.read()

        frontmatter, body = frontmatter_split(content)
        assert frontmatter, f"no frontmatter in {path}"
        fm = yaml.safe_load(frontmatter[len("---\n") : -len("---\n")])

        existing = fm.get("goal")
        # A hand-authored goal is authoritative; only a value this script wrote
        # is re-derived, so a corrected derivation can be re-run in place.
        if not existing or "(backfilled" in existing:
            raw, mark = derive_goal(fm, body)
            value = fit(clean(raw) or clean(str(fm.get("title", ""))), mark)
            if value != existing:
                frontmatter = insert_goal(strip_goal(frontmatter), goal_line(value))
                goals += 1

        missing = missing_sections(body)
        if missing:
            body = backfill_sections(body, missing, path)
            sections += 1

        new_content = frontmatter + body
        if new_content != content:
            with open(path, "w") as f:
                f.write(new_content)

    print(f"goals written {goals}, section-backfilled files {sections}")


if __name__ == "__main__":
    main()
