import { Hono } from "hono";
import { validator } from "hono/validator";
import { execFileSync } from "node:child_process";
import { readFileSync, existsSync } from "node:fs";
import { basename, join } from "node:path";
import {
  doc, freshen, incoming, list, milestoneKey, num, outgoing, query, root, row, rowsOf, slot, str, topicOf,
  type FrontMatter, type Row,
} from "./vault.ts";
import { render, resolve } from "./markdown.ts";

const NOT_SHOWN = new Set(["session"]);
const ordinal = (id: string) => id.match(/^issue\.[^.]+\.(\d{4})\./)?.[1] ?? "";
const newest = (a: { updated: string | null }, b: { updated: string | null }) => (b.updated ?? "").localeCompare(a.updated ?? "");
const done = (s: string | null) => s === "resolved" || s === "abandoned";

const item = (r: Row) => ({
  key: r.id, type: r.type, status: r.status, project: r.project, title: r.title ?? r.id,
  created: r.created, updated: r.updated,
});

const prs = (fm: FrontMatter) =>
  list(fm.external_links).flatMap((u) => {
    const m = String(u).match(/\/pull\/(\d+)/);
    return m ? [{ n: Number(m[1]), url: String(u) }] : [];
  });

function issueRow(r: Row) {
  const { fm } = doc(r);
  return {
    ...item(r),
    ordinal: ordinal(r.id),
    owner: str(fm.owner),
    severity: str(fm.severity),
    verdict: str(fm.verified_verdict),
    prs: prs(fm),
    rounds: num(fm.cost_rounds),
    tokens: num(fm.cost_tokens),
  };
}

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

// An issue names its milestone by full key or by the bare `milestone.<slug>`
// form, which means the issue's own project; both index as `milestone` links.
function members(m: Row): Row[] {
  const bare = `milestone.${m.id.replace(`milestone.${m.project}.`, "")}`;
  const seen = new Map<string, Row>();
  for (const [target, sameProject] of [[m.id, false], [bare, true]] as const) {
    for (const l of incoming(target)) {
      const r = l.relation === "milestone" ? row(l.other) : undefined;
      if (r?.type === "issue" && (!sameProject || r.project === m.project)) seen.set(r.id, r);
    }
  }
  return [...seen.values()];
}

const tally = (rows: { status: string | null }[]) => {
  const c: Record<string, number> = {};
  for (const r of rows) c[r.status ?? "none"] = (c[r.status ?? "none"] ?? 0) + 1;
  return c;
};

function milestoneSummary(m: Row, keep: (i: Row) => boolean = () => true) {
  const { fm, body } = doc(m);
  const issues = members(m);
  const counted = issues.filter((i) => i.status !== "abandoned");
  return {
    ...item(m),
    approved: str(fm.approved),
    done: str(fm.done),
    measured: lastMeasured(body),
    resolved: counted.filter((i) => i.status === "resolved").length,
    total: counted.length,
    counts: tally(issues),
    live: issues.some((i) => i.status === "in-progress"),
    issues: issues.filter(keep).map(issueRow).sort(newest),
  };
}

const described = (r: Row) => {
  const { fm } = doc(r);
  return { ...item(r), description: str(fm.description), confidence: str(fm.confidence), topic: topicOf(r.id) || null };
};

function designs(project: string) {
  return ["product-design", "system-design", "component-design"].flatMap((t) =>
    rowsOf(t)
      .filter((r) => r.project === project && r.status !== "retired")
      .map((r) => ({ ...item(r), diagrams: list(doc(r).fm.diagrams).map(String) })),
  );
}

function projects() {
  const names = new Set(rowsOf("product-design").map((r) => r.project).filter((p): p is string => !!p));
  const issues = rowsOf("issue");
  const ms = rowsOf("milestone");
  return [...names].sort().map((p) => {
    const pd = row(`product-design.${p}`);
    const mine = issues.filter((i) => i.project === p);
    return {
      slug: p,
      description: pd ? str(doc(pd).fm.description) : null,
      inProgress: mine.filter((i) => i.status === "in-progress").length,
      open: mine.filter((i) => i.status === "open").length,
      milestonesInFlight: ms.filter((m) => m.project === p && m.status === "in-progress").length,
      milestonesPlanned: ms.filter((m) => m.project === p && m.status === "planned").length,
      updated: mine.reduce((a, i) => ((i.updated ?? "") > a ? i.updated ?? "" : a), ""),
    };
  });
}

function linked(edges: { other: string; relation: string }[]) {
  const seen = new Map<string, ReturnType<typeof item> & { relation: string }>();
  for (const e of edges) {
    const r = row(e.other);
    if (r && !NOT_SHOWN.has(r.type) && !seen.has(r.id)) seen.set(r.id, { ...item(r), relation: e.relation });
  }
  return [...seen.values()];
}

function crumbs(r: Row, fm: FrontMatter) {
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

// One list per type owns which keys the judge strip shows, so the strip and
// the All properties skip cannot drift apart.
const judgeKeys: Record<string, string[]> = {
  issue: ["verified_verdict", "verified_commit", "verified_at", "cost_rounds", "cost_tokens", "cost_diff", "cost_files", "owner", "severity"],
  learning: ["confidence"],
  decision: ["supersedes", "superseded_by"],
  milestone: ["approved", "done"],
};

type Judged = { label: string; value: string; count?: number; tone?: "pass" | "fail" | "dim"; href?: string; mono?: boolean; title?: string };

function judge(r: Row, fm: FrontMatter, body: string): Judged[] {
  const out: Judged[] = [];
  if (r.type === "issue") {
    const verdict = str(fm.verified_verdict);
    if (verdict) out.push({ label: "Verdict", value: verdict, tone: verdict === "pass" ? "pass" : "fail" });
    const commit = str(fm.verified_commit);
    if (commit) out.push({ label: "Verified at", value: commit.slice(0, 7), mono: true, title: str(fm.verified_at) ?? undefined });
    for (const p of prs(fm)) out.push({ label: "PR", value: `#${p.n}`, href: p.url });
    const rounds = num(fm.cost_rounds);
    if (rounds != null) out.push({ label: "Review rounds", value: String(rounds) });
    const tokens = num(fm.cost_tokens);
    if (tokens != null) out.push({ label: "Tokens", value: String(tokens), count: tokens, title: `${tokens} tokens` });
    const diff = num(fm.cost_diff);
    const files = num(fm.cost_files);
    if (diff != null) out.push({ label: "Change", value: `${diff} lines${files ? ` · ${files} files` : ""}` });
    for (const [k, label] of [["owner", "Owner"], ["severity", "Severity"]] as const) {
      const v = str(fm[k]);
      if (v) out.push({ label, value: v });
    }
  }
  const confidence = str(fm.confidence);
  if (r.type === "learning" && confidence) out.push({ label: "Confidence", value: confidence });
  if (r.type === "decision") {
    for (const k of ["supersedes", "superseded_by"] as const)
      for (const v of list(fm[k])) {
        const ref = resolve(slot(v));
        out.push({ label: k === "supersedes" ? "Supersedes" : "Superseded by", value: ref.label, href: ref.resolved ? `/a/${ref.key}` : undefined });
      }
  }
  if (r.type === "milestone") {
    for (const [k, label] of [["approved", "Approved"], ["done", "Done"]] as const) {
      const v = str(fm[k]);
      if (v) out.push({ label, value: v });
    }
    const m = lastMeasured(body);
    out.push(m ? { label: "Measured", value: m } : { label: "Measured", value: "not measured", tone: "dim" });
  }
  return out;
}

function props(type: string, fm: FrontMatter) {
  const skip = new Set(["title", "description", "type", "status", "project", "tags", "related", "diagrams", ...(judgeKeys[type] ?? [])]);
  return Object.entries(fm).flatMap(([k, v]) => {
    if (skip.has(k)) return [];
    // PR links show in the judge strip; every other external link stays here.
    const vals = list(type === "issue" && k === "external_links" ? list(v).filter((u) => !/\/pull\/\d+/.test(String(u))) : v)
      .map((x) => (x instanceof Date ? x.toISOString().slice(0, 10) : typeof x === "object" && x ? JSON.stringify(x) : String(x)))
      .filter((x) => x !== "");
    return vals.length ? [{ k, v: vals }] : [];
  });
}

// Depth-1 neighbourhood of key, sessions dropped, capped at 60 nodes.
function graph(key: string) {
  const out = linked(outgoing(key)).map((n) => ({ n, edge: { source: key, target: n.key, relation: n.relation } }));
  const inn = linked(incoming(key)).map((n) => ({ n, edge: { source: n.key, target: key, relation: n.relation } }));
  const nodes = new Map([[key, item(row(key)!)]]);
  const edges: { source: string; target: string; relation: string }[] = [];
  for (const { n, edge } of [...out, ...inn]) {
    if (nodes.size >= 60 && !nodes.has(n.key)) continue;
    nodes.set(n.key, item(row(n.key)!));
    edges.push(edge);
  }
  return { nodes: [...nodes.values()], edges };
}

// `anvil hydrate` exits non-zero when the walk meets a broken edge but still
// prints the manifest; stdout holds the layers, stderr the broken edges.
function hydrate(issueKey: string) {
  let out: string;
  let err = "";
  try {
    out = execFileSync("anvil", ["hydrate", issueKey, "--tldr"], { encoding: "utf8", maxBuffer: 64 << 20, stdio: ["ignore", "pipe", "pipe"] });
  } catch (e) {
    const x = e as { stdout?: string; stderr?: string };
    if (!x.stdout?.includes("=== end manifest ===")) throw e;
    out = x.stdout;
    err = x.stderr ?? "";
  }
  const block = out.split("=== end manifest ===")[0];
  const keys = [...block.matchAll(/^\s{2}(\S+)\s+(\S+)\s+\(([^)]*)\)\s*$/gm)].map(([, type, id]) =>
    id.startsWith(`${type}.`) ? id : `${type}.${id}`,
  );
  const broken = err.split("\n").filter((l) => /^\s{2}\S/.test(l)).map((l) => l.trim());
  return { keys, broken };
}

const typeWeight = ["project", "milestone", "issue", "product-design", "system-design", "component-design", "topic", "decision", "thread", "learning", "convention", "inbox"];
const weight = (t: string) => { const i = typeWeight.indexOf(t); return i < 0 ? 99 : i; };

void render("```ts\nconst warm = 1;\n```");

export const api = new Hono()
  .use("*", async (_c, next) => { freshen(); await next(); })

  .get("/nav", (c) => c.json({ projects: projects() }))

  .get("/home", (c) => {
    const live = rowsOf("milestone")
      .filter((m) => m.status === "in-progress")
      .map((m) => milestoneSummary(m, (i) => i.status === "in-progress" || i.status === "escalated"))
      .sort((a, b) => Number(b.live) - Number(a.live) || newest(a, b));
    const shown = new Set(live.flatMap((m) => m.issues.map((i) => i.key)));
    const loose = rowsOf("issue").filter((i) => (i.status === "in-progress" || i.status === "escalated") && !shown.has(i.id)).map(issueRow);
    const recent = [...rowsOf("decision"), ...rowsOf("learning")].sort(newest).slice(0, 8).map(described);
    return c.json({ projects: projects(), milestones: live, loose, recent });
  })

  .get("/project/:slug", (c) => {
    const p = c.req.param("slug");
    const issues = rowsOf("issue").filter((i) => i.project === p);
    if (!issues.length && !row(`product-design.${p}`)) return c.json({ error: "not found" }, 404);
    const ms = rowsOf("milestone").filter((m) => m.project === p);
    const order: Record<string, number> = { "in-progress": 0, planned: 1 };
    const open = ms
      .filter((m) => m.status === "in-progress" || m.status === "planned")
      .map((m) => milestoneSummary(m))
      .sort((a, b) => Number(b.live) - Number(a.live) || order[a.status!] - order[b.status!] || newest(a, b));
    const homed = new Set(open.flatMap((m) => m.issues.map((i) => i.key)));
    const loose = issues.filter((i) => !done(i.status) && !homed.has(i.id)).map(issueRow);
    const finished = ms
      .filter((m) => m.status === "done")
      .sort((a, b) => (str(doc(b).fm.done) ?? b.updated ?? "").localeCompare(str(doc(a).fm.done) ?? a.updated ?? ""))
      .slice(0, 6)
      .map((m) => milestoneSummary(m, () => false));
    const pd = row(`product-design.${p}`);
    const mine = (t: string) => rowsOf(t).filter((d) => d.project === p);
    return c.json({
      slug: p,
      description: pd ? str(doc(pd).fm.description) : null,
      counts: { issues: tally(issues), milestones: tally(ms), decisions: mine("decision").length, learnings: mine("learning").length },
      milestones: open,
      loose,
      done: finished,
      designs: designs(p),
      decisions: mine("decision").slice(0, 6).map(described),
      learnings: mine("learning").slice(0, 6).map(described),
    });
  })

  .get("/artifact/:key", async (c) => {
    const r = row(c.req.param("key"));
    if (!r || NOT_SHOWN.has(r.type)) return c.json({ error: "not found" }, 404);
    const { fm, body } = doc(r);
    const { html, toc } = await render(body);
    return c.json({
      ...item(r),
      description: str(fm.description),
      tags: list(fm.tags).map(String),
      html,
      toc,
      crumbs: crumbs(r, fm),
      judge: judge(r, fm, body),
      props: props(r.type, fm),
      diagrams: list(fm.diagrams).map(String),
      backlinks: linked(incoming(r.id)).sort(newest),
      forward: linked(outgoing(r.id)),
      graph: graph(r.id),
      issues: r.type === "milestone" ? members(r).map(issueRow).sort(newest) : null,
    });
  })

  .get("/stack/:key", async (c) => {
    const key = c.req.param("key");
    const issue = row(key);
    if (!issue || issue.type !== "issue") return c.json({ error: "not found" }, 404);
    const { keys, broken } = hydrate(key);
    const layers = [];
    for (const k of keys) {
      const r = row(k);
      // The view drops nothing: a node hydrate walked but the index lacks is a stub.
      if (!r) { layers.push({ key: k, type: k.split(".")[0], status: null, project: null, title: k, created: null, updated: null, bytes: 0, html: "", indexed: false }); continue; }
      const { body } = doc(r);
      layers.push({ ...item(r), bytes: Buffer.byteLength(body), html: (await render(body)).html, indexed: true });
    }
    return c.json({ key, title: issue.title ?? key, layers, broken });
  })

  .get("/list/:type", validator("query", (q) => ({
    project: typeof q.project === "string" ? q.project : undefined,
    status: typeof q.status === "string" ? q.status : undefined,
    topic: typeof q.topic === "string" ? q.topic : undefined,
  })), (c) => {
    const type = c.req.param("type");
    let rows = NOT_SHOWN.has(type) ? [] : rowsOf(type);
    if (!rows.length) return c.json({ error: "not found" }, 404);
    const { project, status, topic } = c.req.valid("query");
    if (project) rows = rows.filter((r) => r.project === project);
    if (topic) rows = rows.filter((r) => topicOf(r.id) === topic);
    const counts = tally(rows);
    if (status) rows = rows.filter((r) => r.status === status);
    const shown = rows.slice(0, 400);
    return c.json({
      type,
      counts,
      projects: [...new Set(rowsOf(type).map((r) => r.project).filter((p): p is string => !!p))].sort(),
      rows: shown.map(item),
      issues: type === "issue" ? shown.map(issueRow) : [],
      total: rows.length,
    });
  })

  .get("/search", (c) => {
    // NUL ends an FTS string early; every other character is quoted literal text.
    const q = (c.req.query("q") ?? "").replaceAll("\0", " ").trim();
    if (!q) return c.json({ q, hits: [] });
    const match = q.split(/\s+/).map((t) => `"${t.replaceAll('"', '""')}"`).join(" ");
    const hits = query<{ id: string; snip: string }>(
      `select id, snippet(artifact_fts, 3, char(1), char(2), '…', 14) as snip
         from artifact_fts where artifact_fts match ? and type != 'session' order by rank limit 60`,
      match,
    ).flatMap((h) => {
      const r = row(h.id);
      return r ? [{ ...item(r), snippet: h.snip }] : [];
    });
    return c.json({ q, hits });
  })

  .get("/palette", (c) => {
    const entries = projects().map((p) => ({ key: p.slug, type: "project", title: p.slug, status: null as string | null, href: `/p/${p.slug}`, project: p.slug as string | null, updated: p.updated as string | null }));
    for (const r of query<Row>("select * from artifacts where type != 'session' order by updated desc"))
      entries.push({ key: r.id, type: r.type, title: r.title ?? r.id, status: r.status, href: `/a/${r.id}`, project: r.project, updated: r.updated });
    entries.sort((a, b) => weight(a.type) - weight(b.type) || newest(a, b));
    return c.json(entries);
  })

  .get("/knowledge", (c) => {
    const type = c.req.query("type");
    if (type && !["decision", "learning", "thread"].includes(type)) return c.json({ error: "not found" }, 404);
    const rows = (type ? [type] : ["decision", "learning", "thread"]).flatMap(rowsOf)
      .filter((r) => r.status !== "retracted")
      .sort(newest)
      .slice(0, 150)
      .map(described);
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
      counts: { decision: tally(rowsOf("decision")), learning: tally(rowsOf("learning")), thread: tally(rowsOf("thread")) },
    });
  });

export type Api = typeof api;

// Same policy as internal/ui's diagramCSP: inline styles, own-origin fonts and
// data images only. It is the second wall behind the no-<script> refusal.
const diagramCSP = "default-src 'none'; style-src 'unsafe-inline'; font-src 'self'; img-src data:";

export const diagrams = new Hono().get("/:name", (c) => {
  const name = basename(c.req.param("name")).replace(/\.html$/, "");
  const path = join(root, "_meta", "diagrams", `${name}.html`);
  if (!existsSync(path)) return c.text("not found", 404);
  const html = readFileSync(path, "utf8");
  if (/<script/i.test(html)) return c.text("diagram holds a script", 415);
  c.header("Content-Security-Policy", diagramCSP);
  return c.html(html);
});
