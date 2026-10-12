import { Hono } from "hono";
import { execFileSync } from "node:child_process";
import { readFileSync, existsSync } from "node:fs";
import { basename, join } from "node:path";
import {
  all, doc, freshen, incoming, milestoneKey, outgoing, root, row, rowsOf, slot, topicOf, type Row,
} from "./vault.ts";
import { render, resolve } from "./markdown.ts";

const NOT_SHOWN = new Set(["session"]);
const ordinal = (id: string) => id.match(/^issue\.[^.]+\.(\d{4})\./)?.[1] ?? "";

const item = (r: Row) => ({
  key: r.id, type: r.type, status: r.status, project: r.project, title: r.title ?? r.id,
  created: r.created, updated: r.updated,
});
type Item = ReturnType<typeof item>;

const num = (v: unknown) => (typeof v === "number" ? v : Number.isFinite(Number(v)) && v !== "" && v != null ? Number(v) : null);
const prs = (fm: Record<string, any>) =>
  ((fm.external_links ?? []) as unknown[]).flatMap((u) => {
    const m = String(u).match(/\/pull\/(\d+)/);
    return m ? [{ n: Number(m[1]), url: String(u) }] : [];
  });

function issueRow(r: Row) {
  const { fm } = doc(r);
  return {
    ...item(r),
    ordinal: ordinal(r.id),
    owner: fm.owner ?? null,
    severity: fm.severity ?? null,
    verdict: (fm.verified_verdict ?? null) as string | null,
    prs: prs(fm),
    rounds: num(fm.cost_rounds),
    tokens: num(fm.cost_tokens),
    diff: num(fm.cost_diff),
    files: num(fm.cost_files),
    goal: fm.goal ?? fm.description ?? null,
  };
}
export type IssueRow = ReturnType<typeof issueRow>;

// The `## Status` section's last `Measured:` line is a milestone's acceptance verdict.
function lastMeasured(body: string): string | null {
  let inStatus = false;
  let got: string | null = null;
  for (const line of body.split("\n")) {
    const h = line.match(/^## (.*)/);
    if (h) { inStatus = h[1].trim() === "Status"; continue; }
    const m = inStatus && line.match(/^Measured:(.*)/);
    if (m) got = m[1].replaceAll("`", "").trim();
  }
  return got;
}

function milestoneOfIssue(): Map<string, string> {
  const out = new Map<string, string>();
  for (const l of all("select source, target from links where relation = 'milestone'")) out.set(l.source, l.target);
  return out;
}

function membership(project: string | null) {
  const of = milestoneOfIssue();
  const byMs = new Map<string, Row[]>();
  for (const i of rowsOf("issue")) {
    if (project && i.project !== project) continue;
    const t = of.get(i.id);
    if (!t) continue;
    const k = row(t) ? t : milestoneKey(i.project ?? "", t);
    if (!byMs.has(k)) byMs.set(k, []);
    byMs.get(k)!.push(i);
  }
  return byMs;
}

const tally = (rows: { status: string | null }[]) => {
  const c: Record<string, number> = {};
  for (const r of rows) c[r.status ?? "none"] = (c[r.status ?? "none"] ?? 0) + 1;
  return c;
};

function milestoneSummary(m: Row, issues: Row[]) {
  const { fm, body } = doc(m);
  const counted = issues.filter((i) => i.status !== "abandoned");
  const rows = issues.map(issueRow).sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""));
  return {
    ...item(m),
    goal: fm.goal ?? fm.description ?? null,
    approved: fm.approved ?? null,
    done: fm.done ?? null,
    measured: lastMeasured(body),
    resolved: counted.filter((i) => i.status === "resolved").length,
    total: counted.length,
    counts: tally(issues),
    live: issues.some((i) => i.status === "in-progress"),
    issues: rows,
  };
}

function designs(project: string) {
  return ["product-design", "system-design", "component-design"].flatMap((t) =>
    rowsOf(t)
      .filter((r) => r.project === project && r.status !== "retired")
      .map((r) => ({ ...item(r), description: doc(r).fm.description ?? null, diagrams: (doc(r).fm.diagrams ?? []) as string[] })),
  );
}

const withDesc = (r: Row) => {
  const { fm } = doc(r);
  return { ...item(r), description: fm.description ?? null, confidence: fm.confidence ?? null };
};

function projects() {
  const names = new Set(rowsOf("product-design").map((r) => r.project).filter(Boolean) as string[]);
  const issues = rowsOf("issue");
  const ms = rowsOf("milestone");
  return [...names].sort().map((p) => {
    const pd = row(`product-design.${p}`);
    const mine = issues.filter((i) => i.project === p);
    return {
      slug: p,
      description: pd ? doc(pd).fm.description ?? null : null,
      inProgress: mine.filter((i) => i.status === "in-progress").length,
      open: mine.filter((i) => i.status === "open").length,
      milestonesInFlight: ms.filter((m) => m.project === p && m.status === "in-progress").length,
      milestonesPlanned: ms.filter((m) => m.project === p && m.status === "planned").length,
      updated: mine.reduce((a, i) => ((i.updated ?? "") > a ? i.updated ?? "" : a), ""),
    };
  });
}

function backlinks(key: string) {
  const seen = new Set<string>();
  const out: (Item & { relation: string })[] = [];
  for (const l of incoming(key)) {
    const r = row(l.source);
    if (!r || NOT_SHOWN.has(r.type) || seen.has(r.id)) continue;
    seen.add(r.id);
    out.push({ ...item(r), relation: l.relation });
  }
  return out.sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""));
}

function forward(key: string) {
  const seen = new Set<string>();
  const out: (Item & { relation: string })[] = [];
  for (const l of outgoing(key)) {
    const r = row(l.target);
    if (!r || NOT_SHOWN.has(r.type) || seen.has(r.id)) continue;
    seen.add(r.id);
    out.push({ ...item(r), relation: l.relation });
  }
  return out;
}

function crumbs(r: Row, fm: Record<string, any>) {
  const out: { label: string; href: string }[] = [];
  if (r.project) out.push({ label: r.project, href: `/p/${r.project}` });
  if (r.type === "issue" && fm.milestone) {
    const k = slot(fm.milestone);
    const m = row(k) ?? row(milestoneKey(r.project ?? "", k));
    if (m) out.push({ label: m.title ?? m.id, href: `/a/${m.id}` });
  } else if (!r.project) {
    out.push({ label: "Knowledge", href: "/knowledge" });
  }
  return out;
}

function judge(r: Row, fm: Record<string, any>, body: string) {
  const out: { label: string; value: string; tone?: string; href?: string; mono?: boolean; title?: string }[] = [];
  if (r.type === "issue") {
    if (fm.verified_verdict)
      out.push({ label: "Verdict", value: String(fm.verified_verdict), tone: fm.verified_verdict === "pass" ? "pass" : "fail" });
    if (fm.verified_commit) out.push({ label: "Verified at", value: String(fm.verified_commit).slice(0, 7), mono: true, title: String(fm.verified_at ?? "") });
    for (const p of prs(fm)) out.push({ label: "PR", value: `#${p.n}`, href: p.url });
    if (num(fm.cost_rounds) != null) out.push({ label: "Review rounds", value: String(fm.cost_rounds) });
    if (num(fm.cost_tokens) != null) out.push({ label: "Tokens", value: compact(num(fm.cost_tokens)!), title: Number(fm.cost_tokens).toLocaleString("en") });
    if (num(fm.cost_diff) != null) out.push({ label: "Change", value: `${fm.cost_diff} lines${fm.cost_files ? ` · ${fm.cost_files} files` : ""}` });
    if (fm.owner) out.push({ label: "Owner", value: String(fm.owner) });
    if (fm.severity) out.push({ label: "Severity", value: String(fm.severity) });
  }
  if (r.type === "learning" && fm.confidence) out.push({ label: "Confidence", value: String(fm.confidence) });
  if (r.type === "decision") {
    for (const k of ["supersedes", "superseded_by"] as const)
      for (const v of [fm[k]].flat().filter(Boolean)) {
        const ref = resolve(slot(v));
        out.push({ label: k === "supersedes" ? "Supersedes" : "Superseded by", value: ref.label, href: ref.resolved ? `/a/${ref.key}` : undefined });
      }
  }
  if (r.type === "milestone") {
    if (fm.approved) out.push({ label: "Approved", value: String(fm.approved) });
    if (fm.done) out.push({ label: "Done", value: String(fm.done) });
    const m = lastMeasured(body);
    out.push({ label: "Measured", value: m ?? "not measured", tone: m ? undefined : "dim", mono: !!m });
  }
  return out;
}

export function compact(n: number) {
  if (n >= 1e6) return `${(n / 1e6).toFixed(n >= 1e7 ? 0 : 1)}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(n >= 1e4 ? 0 : 1)}k`;
  return String(n);
}

// Depth-1 neighbourhood plus the links among the neighbours, sessions dropped.
function graph(key: string) {
  const nodes = new Map<string, Item>();
  const center = row(key)!;
  nodes.set(key, item(center));
  const edges: { source: string; target: string; relation: string }[] = [];
  for (const l of outgoing(key)) {
    const r = row(l.target);
    if (r && !NOT_SHOWN.has(r.type)) { nodes.set(r.id, item(r)); edges.push({ source: key, target: r.id, relation: l.relation }); }
  }
  for (const l of incoming(key)) {
    const r = row(l.source);
    if (r && !NOT_SHOWN.has(r.type)) { nodes.set(r.id, item(r)); edges.push({ source: r.id, target: key, relation: l.relation }); }
  }
  const capped = [...nodes.values()].slice(0, 60);
  const keep = new Set(capped.map((n) => n.key));
  return { nodes: capped, edges: edges.filter((e) => keep.has(e.source) && keep.has(e.target)) };
}

function props(fm: Record<string, any>) {
  const skip = new Set(["title", "description", "type", "status", "project", "tags", "related", "diagrams", "external_links"]);
  return Object.entries(fm)
    .filter(([k, v]) => !skip.has(k) && v != null && v !== "" && !(Array.isArray(v) && !v.length))
    .map(([k, v]) => ({ k, v: Array.isArray(v) ? v.map(String) : typeof v === "object" ? JSON.stringify(v) : String(v) }));
}

function manifest(issueKey: string) {
  const out = execFileSync("anvil", ["hydrate", issueKey.replace(/^issue\./, ""), "--tldr"], { encoding: "utf8", maxBuffer: 64 << 20 });
  const block = out.split("=== end manifest ===")[0];
  return [...block.matchAll(/^\s{2}(\S+)\s+(\S+)\s+\(([^)]*)\)\s*$/gm)].map(([, type, id]) =>
    id.startsWith(`${type}.`) ? id : `${type}.${id}`,
  );
}

const typeWeight = ["project", "milestone", "issue", "product-design", "system-design", "component-design", "topic", "decision", "thread", "learning", "convention", "inbox"];

export const api = new Hono();
void render("```ts\nconst warm = 1;\n```");

api.use("*", async (_c, next) => { freshen(); await next(); });

api.get("/nav", (c) => c.json({ projects: projects() }));

api.get("/home", (c) => {
  const byMs = membership(null);
  const live = rowsOf("milestone")
    .filter((m) => m.status === "in-progress")
    .map((m) => {
      const s = milestoneSummary(m, byMs.get(m.id) ?? []);
      return { ...s, issues: s.issues.filter((i) => i.status === "in-progress" || i.status === "escalated") };
    })
    .sort((a, b) => Number(b.live) - Number(a.live) || (b.updated ?? "").localeCompare(a.updated ?? ""));
  const inMs = new Set(live.flatMap((m) => m.issues.map((i) => i.key)));
  const loose = rowsOf("issue").filter((i) => i.status === "in-progress" && !inMs.has(i.id)).map(issueRow);
  const recent = [...rowsOf("decision"), ...rowsOf("learning")]
    .sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""))
    .slice(0, 8)
    .map(withDesc);
  return c.json({ projects: projects(), milestones: live, loose, recent });
});

api.get("/project/:slug", (c) => {
  const p = c.req.param("slug");
  if (!rowsOf("issue").some((r) => r.project === p) && !row(`product-design.${p}`)) return c.json({ error: "not found" }, 404);
  const byMs = membership(p);
  const ms = rowsOf("milestone").filter((m) => m.project === p);
  const order = { "in-progress": 0, planned: 1 } as Record<string, number>;
  const open = ms
    .filter((m) => m.status === "in-progress" || m.status === "planned")
    .map((m) => milestoneSummary(m, byMs.get(m.id) ?? []))
    .sort((a, b) => Number(b.live) - Number(a.live) || order[a.status!] - order[b.status!] || (b.updated ?? "").localeCompare(a.updated ?? ""));
  const openKeys = new Set(open.map((m) => m.key));
  const homed = new Set([...byMs.entries()].filter(([k]) => openKeys.has(k)).flatMap(([, v]) => v.map((i) => i.id)));
  const loose = rowsOf("issue")
    .filter((i) => i.project === p && (i.status === "open" || i.status === "in-progress") && !homed.has(i.id))
    .map(issueRow);
  const done = ms
    .filter((m) => m.status === "done")
    .sort((a, b) => String(doc(b).fm.done ?? b.updated).localeCompare(String(doc(a).fm.done ?? a.updated)))
    .slice(0, 6)
    .map((m) => { const s = milestoneSummary(m, byMs.get(m.id) ?? []); return { ...s, issues: [] }; });
  const pd = row(`product-design.${p}`);
  const issues = rowsOf("issue").filter((i) => i.project === p);
  return c.json({
    slug: p,
    description: pd ? doc(pd).fm.description ?? null : null,
    counts: {
      issues: tally(issues),
      milestones: tally(ms),
      decisions: rowsOf("decision").filter((d) => d.project === p).length,
      learnings: rowsOf("learning").filter((d) => d.project === p).length,
    },
    milestones: open,
    loose,
    done,
    designs: designs(p),
    decisions: rowsOf("decision").filter((d) => d.project === p).slice(0, 6).map(withDesc),
    learnings: rowsOf("learning").filter((d) => d.project === p).slice(0, 6).map(withDesc),
  });
});

api.get("/artifact/:key", async (c) => {
  const key = c.req.param("key");
  const r = row(key);
  if (!r || NOT_SHOWN.has(r.type)) return c.json({ error: "not found" }, 404);
  const { fm, body } = doc(r);
  const { html, toc } = await render(body);
  let issues: IssueRow[] | undefined;
  if (r.type === "milestone") {
    const mine = membership(r.project).get(r.id) ?? [];
    issues = mine.map(issueRow).sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""));
  }
  return c.json({
    ...item(r),
    description: fm.description ?? null,
    goal: fm.goal ?? null,
    tags: fm.tags ?? [],
    html,
    toc,
    crumbs: crumbs(r, fm),
    judge: judge(r, fm, body),
    props: props(fm),
    diagrams: (fm.diagrams ?? []) as string[],
    backlinks: backlinks(r.id),
    forward: forward(r.id),
    graph: graph(r.id),
    issues,
  });
});

api.get("/stack/:key", async (c) => {
  const key = c.req.param("key");
  if (!row(key) || !key.startsWith("issue.")) return c.json({ error: "not found" }, 404);
  const layers = [];
  for (const k of manifest(key)) {
    const r = row(k);
    if (!r) continue;
    const { body } = doc(r);
    layers.push({ ...item(r), bytes: Buffer.byteLength(body), html: (await render(body)).html });
  }
  return c.json({ key, title: row(key)!.title, layers });
});

api.get("/list/:type", (c) => {
  const type = c.req.param("type");
  if (NOT_SHOWN.has(type)) return c.json({ error: "not found" }, 404);
  const { project, status, topic } = c.req.query();
  let rows = rowsOf(type);
  if (!rows.length) return c.json({ error: "not found" }, 404);
  if (project) rows = rows.filter((r) => r.project === project);
  if (topic) rows = rows.filter((r) => topicOf(r.id) === topic);
  const counts = tally(rows);
  if (status) rows = rows.filter((r) => r.status === status);
  const projectsSeen = [...new Set(rowsOf(type).map((r) => r.project).filter(Boolean))].sort();
  return c.json({
    type, counts, projects: projectsSeen,
    rows: type === "issue" ? rows.slice(0, 400).map(issueRow) : rows.slice(0, 400).map(item),
    total: rows.length,
  });
});

api.get("/search", (c) => {
  const q = (c.req.query("q") ?? "").trim();
  if (!q) return c.json({ q, hits: [] });
  // Quote every term so FTS operators in user input are literal text.
  const match = q.split(/\s+/).map((t) => `"${t.replaceAll('"', '""')}"`).join(" ");
  const hits = all(
    `select id, type, snippet(artifact_fts, 3, '\u0001', '\u0002', '…', 14) as snip
       from artifact_fts where artifact_fts match ? and type != 'session' order by rank limit 60`,
    match,
  ).map((h) => {
    const r = row(h.id);
    return r && { ...item(r), snippet: String(h.snip) };
  }).filter(Boolean);
  return c.json({ q, hits });
});

api.get("/palette", (c) => {
  const entries: { key: string; type: string; title: string; status: string | null; href: string; project: string | null; updated: string | null }[] = [];
  for (const p of projects()) entries.push({ key: p.slug, type: "project", title: p.slug, status: null, href: `/p/${p.slug}`, project: p.slug, updated: p.updated });
  const rows = all("select * from artifacts where type not in ('session') order by updated desc") as Row[];
  for (const r of rows) entries.push({ key: r.id, type: r.type, title: r.title ?? r.id, status: r.status, href: `/a/${r.id}`, project: r.project, updated: r.updated });
  const w = (t: string) => { const i = typeWeight.indexOf(t); return i < 0 ? 99 : i; };
  entries.sort((a, b) => w(a.type) - w(b.type) || (b.updated ?? "").localeCompare(a.updated ?? ""));
  return c.json(entries);
});

api.get("/knowledge", (c) => {
  const type = c.req.query("type");
  const types = type ? [type] : ["decision", "learning", "thread"];
  const rows = types.flatMap(rowsOf)
    .filter((r) => r.status !== "retracted")
    .sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""))
    .slice(0, 150)
    .map((r) => ({ ...withDesc(r), topic: topicOf(r.id) || null }));
  const topics = new Map<string, { slug: string; decisions: number; threads: number; open: number; moved: string }>();
  for (const r of [...rowsOf("decision"), ...rowsOf("thread")]) {
    const t = topicOf(r.id);
    if (!t) continue;
    const e = topics.get(t) ?? { slug: t, decisions: 0, threads: 0, open: 0, moved: "" };
    if (r.type === "decision") e.decisions++; else e.threads++;
    if (r.type === "thread" && r.status === "open") e.open++;
    if ((r.updated ?? "") > e.moved) e.moved = r.updated ?? "";
    topics.set(t, e);
  }
  return c.json({
    rows,
    topics: [...topics.values()].filter((t) => t.threads || t.decisions > 1).sort((a, b) => b.moved.localeCompare(a.moved)),
    counts: {
      decision: tally(rowsOf("decision")),
      learning: tally(rowsOf("learning")),
      thread: tally(rowsOf("thread")),
    },
  });
});

export const diagrams = new Hono();
// Diagrams are self-contained HTML. They render in a sandboxed frame with no
// script; a file that holds a script tag is refused outright.
diagrams.get("/:name", (c) => {
  const name = basename(c.req.param("name")).replace(/\.html$/, "");
  const path = join(root, "_meta", "diagrams", `${name}.html`);
  if (!existsSync(path)) return c.text("not found", 404);
  const html = readFileSync(path, "utf8");
  if (/<script/i.test(html)) return c.text("diagram holds a script", 415);
  c.header("Content-Security-Policy", "script-src 'none'; default-src 'self' 'unsafe-inline' data:");
  return c.html(html);
});
