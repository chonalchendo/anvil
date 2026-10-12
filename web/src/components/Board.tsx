import { Link } from "@tanstack/react-router";
import { compact, type IssueRow } from "../lib/api";
import { Glyph } from "./ui/Glyph";
import { Pill } from "./ui/Pill";

const columns = [
  { status: "open", label: "Open" },
  { status: "in-progress", label: "In progress" },
  { status: "escalated", label: "Escalated" },
  { status: "resolved", label: "Resolved" },
];

// GitHub Projects' board: position encodes state, so state reads by location.
// Card anatomy puts identity (ordinal) above the title, then the judge pills.
export function Board({ rows }: { rows: IssueRow[] }) {
  return (
    <div className="grid grid-cols-4 gap-3">
      {columns.map((col) => {
        const cards = rows.filter((r) => r.status === col.status).sort((a, b) => (b.updated ?? "").localeCompare(a.updated ?? ""));
        return (
          <section key={col.status} aria-label={col.label} className="flex min-w-0 flex-col rounded-lg border border-line bg-panel">
            <header className="flex h-10 items-center gap-2 border-b border-line px-3">
              <Glyph status={col.status} />
              <span className="text-sm font-medium text-fg">{col.label}</span>
              <span className="rounded-full bg-raised px-1.5 text-xs text-dim num">{cards.length}</span>
            </header>
            <div className="flex max-h-[640px] flex-col gap-2 overflow-y-auto overscroll-contain p-2">
              {cards.slice(0, 40).map((r) => (
                <Link key={r.key} to="/a/$key" params={{ key: r.key }} className="flex flex-col gap-1.5 rounded-md border border-line bg-bg p-2.5 hover:border-line-strong">
                  <span className="font-mono text-xs text-faint" translate="no">{r.project} #{r.ordinal}</span>
                  <span className="line-clamp-3 text-sm leading-[18px] text-fg">{r.title}</span>
                  <span className="flex flex-wrap gap-1">
                    {r.verdict && <Pill tone={r.verdict === "pass" ? "pass" : "fail"}>{r.verdict}</Pill>}
                    {r.prs[0] && <Pill>#{r.prs[0].n}</Pill>}
                    {r.tokens != null && <Pill>{compact(r.tokens)} tok</Pill>}
                    {(r.severity === "high" || r.severity === "critical") && <Pill tone="escalated">{r.severity}</Pill>}
                    {r.severity === "low" && <Pill>low</Pill>}
                  </span>
                </Link>
              ))}
              {!cards.length && <div className="px-1 py-2 text-xs text-faint">Empty</div>}
            </div>
          </section>
        );
      })}
    </div>
  );
}
