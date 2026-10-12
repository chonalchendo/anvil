import { Link } from "@tanstack/react-router";
import clsx from "clsx";
import {
  BookMarked, Boxes, CircleDot, Compass, FolderKanban, Hash, Inbox, Lightbulb, MessagesSquare,
  Milestone as MilestoneIcon, Puzzle, Scale, type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";
import { age, compact, type IssueRow, type Item } from "../lib/api";

// Status is shape plus hue at a fixed column (Linear). Shape alone separates
// the states for readers who cannot see the hue.
type Shape = "hollow" | "dashed" | "half" | "check" | "cross" | "slash" | "warn" | "dot" | "pause";
const shapeOf: Record<string, Shape> = {
  open: "hollow", active: "dot", planned: "dashed", proposed: "dashed", draft: "dashed", raw: "dashed",
  "in-progress": "half", paused: "pause",
  resolved: "check", done: "check", accepted: "check", verified: "check", closed: "check", promoted: "check",
  escalated: "warn",
  abandoned: "cross", dropped: "cross", rejected: "cross", retired: "cross", retracted: "cross",
  superseded: "slash", stale: "slash",
};

export function Glyph({ status, size = 14, className }: { status: string | null; size?: number; className?: string }) {
  const shape = shapeOf[status ?? ""] ?? "hollow";
  const c = "var(--s)";
  return (
    <svg
      width={size} height={size} viewBox="0 0 14 14" aria-label={status ?? "no status"} role="img"
      className={clsx(`s-${status ?? "none"} shrink-0`, className)}
    >
      {shape === "warn" ? (
        <path d="M7 1.5 13 12.5H1Z" fill="none" stroke={c} strokeWidth="1.5" strokeLinejoin="round" />
      ) : (
        <circle cx="7" cy="7" r="5.75" fill={["check", "cross", "slash"].includes(shape) ? c : "none"}
          stroke={c} strokeWidth="1.5" strokeDasharray={shape === "dashed" ? "2.2 2" : undefined} />
      )}
      {shape === "half" && <path d="M7 3.5a3.5 3.5 0 0 1 0 7Z" fill={c} />}
      {shape === "dot" && <circle cx="7" cy="7" r="2.5" fill={c} />}
      {shape === "pause" && <path d="M5.5 4.5v5M8.5 4.5v5" stroke={c} strokeWidth="1.5" />}
      {shape === "check" && <path d="m4.4 7.2 1.8 1.8 3.5-3.6" fill="none" stroke="var(--color-bg)" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />}
      {shape === "cross" && <path d="m5 5 4 4m0-4-4 4" stroke="var(--color-bg)" strokeWidth="1.5" strokeLinecap="round" />}
      {shape === "slash" && <path d="m4.8 9.2 4.4-4.4" stroke="var(--color-bg)" strokeWidth="1.5" strokeLinecap="round" />}
      {shape === "warn" && <path d="M7 5.5v3M7 10.2v.1" stroke={c} strokeWidth="1.5" strokeLinecap="round" />}
    </svg>
  );
}

const icons: Record<string, LucideIcon> = {
  project: FolderKanban, milestone: MilestoneIcon, issue: CircleDot, "product-design": Compass,
  "system-design": Boxes, "component-design": Puzzle, decision: Scale, learning: Lightbulb,
  thread: MessagesSquare, inbox: Inbox, convention: BookMarked, topic: Hash,
};
export function TypeIcon({ type, className }: { type: string; className?: string }) {
  const I = icons[type] ?? CircleDot;
  return <I size={14} strokeWidth={1.75} className={clsx("shrink-0 text-faint", className)} aria-hidden />;
}

// Tinted pill: same-hue fill at 12% and border at 30% (GitHub Projects).
export function Pill({ tone, children, title }: { tone?: string; children: ReactNode; title?: string }) {
  return (
    <span
      title={title}
      className={clsx("inline-flex h-[18px] items-center gap-1 rounded-sm border px-1.5 text-cap font-medium num whitespace-nowrap", tone && `s-${tone}`)}
      style={tone ? {
        color: "var(--s)",
        background: "color-mix(in srgb, var(--s) 12%, transparent)",
        borderColor: "color-mix(in srgb, var(--s) 30%, transparent)",
      } : { color: "var(--color-dim)", borderColor: "var(--color-line-strong)" }}
    >
      {children}
    </span>
  );
}

export const Caption = ({ children, className }: { children: ReactNode; className?: string }) => (
  <div className={clsx("text-cap font-medium uppercase tracking-[0.04em] text-faint", className)}>{children}</div>
);

export function ArtifactLink({ item, className, children }: { item: Pick<Item, "key">; className?: string; children?: ReactNode }) {
  return (
    <Link to="/a/$key" params={{ key: item.key }} className={className}>
      {children}
    </Link>
  );
}

// A thin progress meter replaces Grafana's full-saturation tile: area of hue
// for state, the number beside it for the value.
export function Meter({ done, total, live }: { done: number; total: number; live?: boolean }) {
  const pct = total ? Math.round((done / total) * 100) : 0;
  return (
    <span className="inline-flex items-center gap-2" title={`${done} of ${total} resolved`}>
      <span className="relative h-1 w-16 overflow-hidden rounded-full bg-line-strong" aria-hidden>
        <span className={clsx("absolute inset-y-0 left-0 rounded-full", live ? "bg-s-progress" : "bg-s-done")} style={{ width: `${pct}%` }} />
      </span>
      <span className="num text-xs text-dim">{done}/{total}</span>
    </span>
  );
}

// Issue row: glyph column, mono ordinal, title, then judge cells right-aligned
// in tabular numerals so a column of rows reads as a table (Linear + Grafana legend).
export function IssueRows({ rows, showProject, empty }: { rows: IssueRow[]; showProject?: boolean; empty?: string }) {
  if (!rows.length) return <div className="px-3 py-2 text-dim">{empty ?? "No issues."}</div>;
  return (
    <div role="table" aria-label="Issues" className="text-sm">
      {rows.map((r) => (
        <Link
          key={r.key} to="/a/$key" params={{ key: r.key }} role="row"
          className={clsx("group grid h-8 items-center [content-visibility:auto] [contain-intrinsic-size:auto_32px] gap-x-2.5 rounded-md px-3 hover:bg-hover", showProject ? "grid-cols-[16px_88px_minmax(0,1fr)_auto]" : "grid-cols-[16px_36px_minmax(0,1fr)_auto]")}
        >
          <Glyph status={r.status} />
          <span className="font-mono text-xs text-faint num" translate="no">{showProject && <span className="text-dim">{r.project} </span>}{r.ordinal}</span>
          <span className="truncate text-body group-hover:text-fg">{r.title}</span>
          <span className="flex items-center gap-3 text-xs text-dim">
            {r.verdict && <Pill tone={r.verdict === "pass" ? "pass" : "fail"}>{r.verdict === "pass" ? "pass" : r.verdict}</Pill>}
            {r.prs[0] && <span className="num w-10 text-right font-mono" translate="no">#{r.prs[0].n}</span>}
            {r.rounds != null && <span className="num w-8 text-right" title="review rounds">{r.rounds}r</span>}
            {r.tokens != null && <span className="num w-10 text-right" title={`${r.tokens.toLocaleString()} tokens`}>{compact(r.tokens)}</span>}
            {r.status === "in-progress" && r.owner && <span className="max-w-28 truncate text-faint">{r.owner}</span>}
            <span className="num w-9 text-right text-faint" title={r.updated ?? ""}>{age(r.updated)}</span>
          </span>
        </Link>
      ))}
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <div className="rounded-lg border border-dashed border-line-strong px-4 py-6 text-center text-dim">{children}</div>;
}

export function PageHead({ crumbs, title, lede, children }: { crumbs?: ReactNode; title: ReactNode; lede?: ReactNode; children?: ReactNode }) {
  return (
    <header className="mb-8">
      {crumbs && <nav aria-label="Breadcrumb" className="mb-3 flex items-center gap-1.5 text-xs text-dim">{crumbs}</nav>}
      <h1 className="text-xl font-semibold tracking-[-0.012em]">{title}</h1>
      {lede && <p className="mt-2 max-w-[68ch] text-base text-dim">{lede}</p>}
      {children}
    </header>
  );
}

export const Loading = () => <div className="p-10 text-dim" aria-live="polite">Loading…</div>;
