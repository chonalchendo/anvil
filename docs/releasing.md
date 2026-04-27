# Releasing

Read when cutting a new version of Anvil.

## Steps

1. `uv version --bump patch` (or `minor` / `major`).
2. Update `src/anvil/__init__.py` to match the new version.
3. Update the `## Status` section in `README.md` with the new version and a one-line summary of what's included (match the milestone description from the product design).
4. Add a new entry to `CHANGELOG.md` under the new version header with the date and grouped Added/Changed/Fixed notes; add the matching comparison link at the bottom of the file. Keep entries brief, concise, and to the point.
5. Verify the library smoke test passes (loads all enabled methodology skills together against the diverse prompt set).
6. `git add pyproject.toml uv.lock src/anvil/__init__.py README.md CHANGELOG.md`
7. `git commit -m "release: v$(uv version)"`
8. `git tag "v$(uv version)"`
9. `git push && git push --tags`

The `publish.yml` workflow handles build, smoke test, and PyPI upload via trusted publishers.

## When to bump which version

- **Patch** (`0.1.0` → `0.1.1`): bug fixes, internal refactors, doc updates.
- **Minor** (`0.1.0` → `0.2.0`): new commands, new skills, new schemas, new adapters. Backward-compatible additions.
- **Major** (`0.1.0` → `1.0.0`): breaking changes to the CLI surface, schema-incompatible vault changes, adapter ABC signature changes.

Until v1.0.0, treat minor bumps as the place to absorb breaking changes — the project is alpha and users have explicit expectation of churn.