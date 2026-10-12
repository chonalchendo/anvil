import type { Entry } from "./api";

export const typeOrder = ["project", "milestone", "issue", "product-design", "system-design", "component-design", "decision", "thread", "learning", "convention", "inbox"];

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// Exact title or key-segment prefix beats a word prefix beats a substring;
// within a tier the server's order (type weight, then recency) holds. With no
// term, the palette offers what a reader most often jumps to.
export function rank(entries: Entry[], term: string, limit = 60): Entry[] {
  const t = term.toLowerCase().trim();
  if (!t) return entries.filter((e) => e.type === "project" || e.type === "milestone" || e.status === "in-progress").slice(0, 40);
  const words = t.split(/\s+/);
  const wordStart = new RegExp(`\\b${escape(t)}`);
  const scored: [number, number, Entry][] = [];
  entries.forEach((e, i) => {
    const title = e.title.toLowerCase();
    const key = e.key.toLowerCase();
    if (!words.every((w) => title.includes(w) || key.includes(w))) return;
    const tier = title.startsWith(t) || key.split(".").some((p) => p.startsWith(t)) ? 0 : wordStart.test(title) ? 1 : 2;
    scored.push([tier, i, e]);
  });
  return scored.sort((a, b) => a[0] - b[0] || a[1] - b[1]).slice(0, limit).map((s) => s[2]);
}

// Groups keep the order their best hit ranked; with no term, type order.
export function group(hits: Entry[], term: string): [string, Entry[]][] {
  const g = new Map<string, Entry[]>();
  for (const h of hits) g.set(h.type, [...(g.get(h.type) ?? []), h]);
  const out = [...g.entries()];
  return term.trim() ? out : out.sort((a, b) => typeOrder.indexOf(a[0]) - typeOrder.indexOf(b[0]));
}
