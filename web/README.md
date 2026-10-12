# anvil web

A read-only web view of the anvil vault, built from reverse-engineered first principles. `DESIGN.md` says which source each part comes from; `research/` holds the decompositions.

## Run

Needs Node 22.19 or later, pnpm, and `anvil` on `PATH` (the server asks `anvil where` for the vault).

```bash
pnpm install
pnpm dev      # http://127.0.0.1:7790 (Vite) with the API on :7791
pnpm build && pnpm start   # one process on http://127.0.0.1:7790
```

## Shape

- `server/` is a Hono app on loopback. It reads `.anvil/vault.db` read-only, plus the artifact files, and answers GET and HEAD only. It calls `anvil hydrate` for the context stack and runs one `anvil list` at most every 5 s so the CLI reindexes a stale index.
- `src/` is React 19, TanStack Router and Query, Tailwind v4 tokens, Radix primitives, cmdk for ⌘K and d3-force for the local graph.
- Diagrams load in a sandboxed frame with no script. A diagram file that holds a script returns 415.
