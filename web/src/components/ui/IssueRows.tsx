import { Link } from "@tanstack/react-router";
import { age, compact, type IssueRow } from "../../lib/api";
import { cn } from "../../lib/cn";
import { Glyph } from "./Glyph";
import { Pill } from "./Pill";

export interface IssueRowsProps {
  rows: IssueRow[];
  showProject?: boolean;
  empty?: string;
}

// Issue row: glyph column, mono ordinal, title, then judge cells right-aligned
// in tabular numerals so a column of rows reads as a table (Linear + Grafana legend).
export function IssueRows({ rows, showProject, empty = "No issues." }: IssueRowsProps) {
  if (!rows.length) return <div className="px-3 py-2 text-dim">{empty}</div>;
  return (
    <div role="table" aria-label="Issues" className="text-sm">
      {rows.map((r) => (
        <Link
          key={r.key} to="/a/$key" params={{ key: r.key }} role="row"
          className={cn(
            "group grid h-8 items-center gap-x-2.5 rounded-md px-3 [contain-intrinsic-size:auto_32px] [content-visibility:auto] hover:bg-hover",
            showProject ? "grid-cols-[16px_88px_minmax(0,1fr)_auto]" : "grid-cols-[16px_36px_minmax(0,1fr)_auto]",
          )}
        >
          <Glyph status={r.status} />
          <span className="font-mono text-xs text-faint num" translate="no">
            {showProject && <span className="text-dim">{r.project} </span>}
            {r.ordinal}
          </span>
          <span className="truncate text-body group-hover:text-fg">{r.title}</span>
          <span className="flex items-center gap-3 text-xs text-dim">
            {r.verdict && <Pill tone={r.verdict === "pass" ? "pass" : "fail"}>{r.verdict}</Pill>}
            {r.prs[0] && <span className="w-10 text-right font-mono num" translate="no">#{r.prs[0].n}</span>}
            {r.rounds != null && <span className="w-8 text-right num" title="review rounds">{r.rounds}r</span>}
            {r.tokens != null && <span className="w-10 text-right num" title={`${r.tokens.toLocaleString()} tokens`}>{compact(r.tokens)}</span>}
            {r.status === "in-progress" && r.owner && <span className="max-w-28 truncate text-faint">{r.owner}</span>}
            <span className="w-9 text-right text-faint num" title={r.updated ?? ""}>{age(r.updated)}</span>
          </span>
        </Link>
      ))}
    </div>
  );
}
