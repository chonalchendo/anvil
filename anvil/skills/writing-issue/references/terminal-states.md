# Working the issue (state machine) and terminal states

The issue lifecycle is `open → in-progress → resolved` (with `→ abandoned` and reverse audit edges). All status changes go through `anvil transition`, not direct frontmatter edits.

```bash
# Claim — --owner is required (open → in-progress)
anvil transition issue <id> in-progress --owner <name>

# Resolve when the work is merged (in-progress → resolved)
anvil transition issue <id> resolved

# Reopen with audit trail (resolved → open requires --reason)
anvil transition issue <id> open --reason "<why>"
```

Use `anvil set ... status` only as a force-edit escape hatch when `transition` rejects a legal-but-unusual move.

## Terminal states

Three exits:

1. **`issue` created** — file exists, validates, milestone link set. Hand off to `completing-issue` for implementation.
2. **`decision/rejected`** — user bailed mid-session. Prompt: "log this as a rejected decision?" If yes:
   ```bash
   anvil create decision --title "Considered: <X>" --json
   anvil set decision <id> status rejected
   anvil set decision <id> date <today>
   ```
   Decision file lands at `~/anvil-vault/30-decisions/<topic>.<NNNN>-<slug>.md` (MADR-conformant; see your project's decision-doc conventions). Body is one paragraph: what was considered, why rejected. If no, no artifact.
3. **Paused** — user wants to think more. No artifact. If the source was an inbox item, it stays as-is for later resumption.
