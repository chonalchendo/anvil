import { DatabaseSync } from "node:sqlite";
import { execFileSync } from "node:child_process";
import { readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import matter from "gray-matter";

export type Row = {
  id: string;
  type: string;
  status: string | null;
  project: string | null;
  title: string | null;
  path: string;
  created: string | null;
  updated: string | null;
};

export type Doc = { fm: Record<string, any>; body: string };

// The vault root comes from the CLI so this server and `anvil` never disagree.
function vaultRoot(): string {
  if (process.env.ANVIL_VAULT) return process.env.ANVIL_VAULT;
  const out = execFileSync("anvil", ["where"], { encoding: "utf8" });
  const m = out.match(/^vault:\s*(.+)$/m);
  if (!m) throw new Error(`anvil where printed no vault line:\n${out}`);
  return m[1].trim();
}

export const root = vaultRoot();
const db = new DatabaseSync(join(root, ".anvil", "vault.db"), { readOnly: true });

// Any anvil read verb reindexes a stale index; run one at most every 5 s so
// edits an agent makes show up without this server writing the index itself.
let lastFresh = 0;
export function freshen() {
  const now = Date.now();
  if (now - lastFresh < 5000) return;
  lastFresh = now;
  try {
    execFileSync("anvil", ["list", "milestone", "--limit", "1", "--json"], { stdio: "ignore" });
  } catch {
    // A failed freshness probe leaves the last good index in place.
  }
}

const q = (sql: string) => db.prepare(sql);
const byType = q("select * from artifacts where type = ? order by updated desc, id desc");
const byId = q("select * from artifacts where id = ?");
const linksFrom = q("select target, relation from links where source = ?");
const linksTo = q("select source, relation from links where target = ?");

export const rowsOf = (type: string) => byType.all(type) as Row[];
export const row = (id: string) => byId.get(id) as Row | undefined;
export const outgoing = (id: string) => linksFrom.all(id) as { target: string; relation: string }[];
export const incoming = (id: string) => linksTo.all(id) as { source: string; relation: string }[];
export const all = (sql: string, ...args: any[]) => q(sql).all(...args) as any[];

const cache = new Map<string, { mtime: number; doc: Doc }>();
export function doc(r: Row): Doc {
  const mtime = statSync(r.path).mtimeMs;
  const hit = cache.get(r.path);
  if (hit && hit.mtime === mtime) return hit.doc;
  const parsed = matter(readFileSync(r.path, "utf8"));
  const d = { fm: parsed.data, body: parsed.content };
  cache.set(r.path, { mtime, doc: d });
  return d;
}

// A frontmatter wikilink is `[[key]]` or `[[key|label]]`; return the key.
export function slot(v: unknown): string {
  const s = typeof v === "string" ? v : "";
  return s.replace(/^\[\[|\]\]$/g, "").split("|")[0].trim();
}

// A bare `milestone.<slug>` slot means the issue's own project.
export function milestoneKey(project: string, s: string): string {
  const slug = s.replace(/^milestone\./, "");
  return slug.includes(".") ? s : `milestone.${project}.${slug}`;
}

const topicRe = /^(?:thread|decision)\.(.+)\.(\d{4})-/;
export const topicOf = (id: string) => id.match(topicRe)?.[1] ?? "";
