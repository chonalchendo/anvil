#!/usr/bin/env bash
# Lints anvil's own methodology SKILL.md files against the token-budget caps in
# docs/skill-authoring.md. Shared by the CI step and the pre-commit hook so the
# two stay identical (the inline go-file-length gate is one check; description
# extraction is too much for safe inline quoting in two places). Over the hard
# cap fails; over the target warns. ::error/::warning render in GitHub Actions
# and print plainly elsewhere.
set -euo pipefail

max_file=500 warn_file=200 max_desc=1024 warn_desc=250
fail=0

while IFS= read -r f; do
  n=$(wc -l <"$f" | tr -d '[:space:]')
  if [ "$n" -gt "$max_file" ]; then
    echo "::error file=$f::SKILL.md is $n lines (max $max_file, whole file) — extract to references/ or split"
    fail=1
  elif [ "$n" -gt "$warn_file" ]; then
    echo "::warning file=$f::SKILL.md is $n lines (over the $warn_file target, whole file) — consider extracting to references/"
  fi

  desc=$(sed -n 's/^description:[[:space:]]*//p' "$f" | head -1)
  desc=${desc#\"}
  desc=${desc%\"}
  d=${#desc}
  if [ "$d" -gt "$max_desc" ]; then
    echo "::error file=$f::description is $d chars (max $max_desc)"
    fail=1
  elif [ "$d" -gt "$warn_desc" ]; then
    echo "::warning file=$f::description is $d chars (over the $warn_desc practical cap — Claude Code truncates skill listings at 250)"
  fi
done < <(git ls-files 'anvil/skills/*/SKILL.md')

exit "$fail"
