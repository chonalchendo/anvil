#!/usr/bin/env python3
r"""Throwaway backfill script — anvil.0212.136-back-catalogue-issues-missing.

Adds a `goal:` frontmatter field to pre-`goal:`-requirement issues so `anvil
validate` stops flagging `missing_required` findings naming `field: goal`.
Each inserted value is derived from the issue's own content — the first
`## Acceptance criteria` bullet, else the frontmatter `acceptance:` list,
else the first sentence of `## Problem`, else `description`, else `title` —
and is suffixed " (backfilled)" per the issue's non-goal against hand-
authoring 136 meaningful goals. Not shipped code, and not wired into the
binary — it is retained as the audit and reproducibility record for a
135-file mutation to a vault that is only loosely versioned.

`insert_goal` places the new `goal:` line at its alphabetical position among
top-level frontmatter keys (matching the vault's existing convention), using
`yaml.safe_dump` on a single key so any value needing YAML quoting (colons,
leading punctuation) round-trips safely instead of corrupting the document.

Usage: python3 backfill-issue-goals.py <manifest>
manifest: one vault issue path per line, each missing `goal:`. Derive one
live with:

    anvil validate 2>&1 | awk '
      /^\[constraint_violation\]/ { path="" }
      /  path:/ { path=$2 }
      /field: goal/ && path { print path }
    ' | sort -u
"""
import re
import sys

import yaml

MARK = " (backfilled)"
MAX_LEN = 120


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


def section(body, heading):
    # Anchored to line-start so a heading name merely *quoted* in prose
    # (e.g. inside a code span elsewhere in the body) is never mistaken
    # for the real heading.
    m = re.search(r"(?m)^" + re.escape(heading) + r"\s*$", body)
    if not m:
        return None
    rest = body[m.end() :]
    end = re.search(r"\n#{1,3} ", rest)
    return rest[: end.start()] if end else rest


def derive_goal(fm, body):
    acc_section = section(body, "## Acceptance criteria")
    if acc_section:
        bullet = first_bullet(acc_section)
        if bullet:
            return bullet
    acc_list = fm.get("acceptance")
    if isinstance(acc_list, list) and acc_list:
        return str(acc_list[0])
    problem = section(body, "## Problem")
    if problem:
        text = " ".join(problem.split())
        if text:
            m = re.match(r"(.{0,140}?[.!?])(\s|$)", text)
            return m.group(1) if m else text
    if fm.get("description"):
        return str(fm["description"])
    return str(fm.get("title", ""))


def clean(text):
    return " ".join(text.split()).strip().rstrip(".")


def fit(text, suffix):
    budget = MAX_LEN - len(suffix)
    if len(text) <= budget:
        return text + suffix
    truncated = text[: budget - 1].rsplit(" ", 1)[0]
    return truncated + "…" + suffix


def goal_line(value):
    return yaml.safe_dump({"goal": value}, default_flow_style=False, allow_unicode=True).rstrip("\n")


def insert_goal(frontmatter, line):
    lines = frontmatter.splitlines(keepends=True)
    # lines[0] is the opening "---\n", lines[-1] the closing "---\n"; only
    # top-level (unindented "key:") lines are candidate insertion anchors.
    for i in range(1, len(lines) - 1):
        m = re.match(r"^([A-Za-z_]+):", lines[i])
        if m and m.group(1) > "goal":
            return "".join(lines[:i]) + line + "\n" + "".join(lines[i:])
    return "".join(lines[:-1]) + line + "\n" + lines[-1]


def main():
    manifest_path = sys.argv[1]
    with open(manifest_path) as f:
        paths = [line.strip() for line in f if line.strip()]

    changed = []
    for path in paths:
        with open(path) as f:
            content = f.read()

        frontmatter, body = frontmatter_split(content)
        assert frontmatter, f"no frontmatter in {path}"
        fm = yaml.safe_load(frontmatter[len("---\n") : -len("---\n")])
        if fm.get("goal"):
            continue

        raw = derive_goal(fm, body)
        value = fit(clean(raw) or clean(str(fm.get("title", ""))), MARK)
        new_frontmatter = insert_goal(frontmatter, goal_line(value))

        with open(path, "w") as f:
            f.write(new_frontmatter + body)
        changed.append(path)

    print(f"changed {len(changed)} files")


if __name__ == "__main__":
    main()
