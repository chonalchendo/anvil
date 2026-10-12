import * as Collapsible from "@radix-ui/react-collapsible";
import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import type { Milestone } from "../lib/api";
import { cn } from "../lib/cn";
import { Glyph } from "./ui/Glyph";
import { IssueRows } from "./ui/IssueRows";
import { Meter } from "./ui/Meter";
import { Pill } from "./ui/Pill";

const rank: Record<string, number> = { "in-progress": 0, escalated: 1, open: 2, resolved: 3, abandoned: 4 };
const finished = (s: string | null) => s === "resolved" || s === "abandoned";

export interface MilestoneGroupProps {
  milestone: Milestone;
  defaultOpen?: boolean;
  showProject?: boolean;
}

// A milestone fold: the header is its own readout (status, meter, measured),
// the body lists live work first and keeps finished work one click away.
export function MilestoneGroup({ milestone: m, defaultOpen, showProject }: MilestoneGroupProps) {
  const [open, setOpen] = useState(defaultOpen ?? m.live);
  const [showDone, setShowDone] = useState(false);
  const sorted = [...m.issues].sort((a, b) => (rank[a.status ?? ""] ?? 9) - (rank[b.status ?? ""] ?? 9));
  const active = sorted.filter((i) => !finished(i.status));
  const doneCount = sorted.length - active.length;
  return (
    <Collapsible.Root open={open} onOpenChange={setOpen} className="border-b border-line last:border-b-0">
      <div className="flex h-10 items-center gap-2.5 px-3">
        <Collapsible.Trigger aria-label={open ? "Collapse" : "Expand"} className="-ml-1 grid size-5 place-items-center rounded-sm text-faint hover:bg-hover hover:text-fg">
          <ChevronRight size={14} className={cn("transition-transform", open && "rotate-90")} />
        </Collapsible.Trigger>
        <Glyph status={m.status} />
        <Link to="/a/$key" params={{ key: m.key }} className="min-w-0 truncate text-sm font-medium text-fg hover:underline">{m.title}</Link>
        {showProject && m.project && <Pill>{m.project}</Pill>}
        <span className="ml-auto flex shrink-0 items-center gap-4 text-xs text-dim">
          {(["in-progress", "escalated", "open"] as const).map((s) =>
            m.counts[s] ? <span key={s} className="flex items-center gap-1 num"><Glyph status={s} size={11} />{m.counts[s]}</span> : null,
          )}
          <Meter done={m.resolved} total={m.total} isLive={m.live} />
          <span className={cn(m.measured ? "text-dim" : "text-faint")} title={m.measured ?? "No Measured: line yet"}>
            {m.measured ? `measured ${m.measured.slice(0, 10)}` : "not measured"}
          </span>
        </span>
      </div>
      <Collapsible.Content className="pb-2 pl-5">
        <IssueRows rows={showDone ? sorted : active} empty={doneCount ? "No open issues." : "No issues yet."} />
        {doneCount > 0 && (
          <button onClick={() => setShowDone((s) => !s)} className="ml-3 mt-1 text-xs text-faint hover:text-dim">
            {showDone ? "Hide" : "Show"} {doneCount} finished
          </button>
        )}
      </Collapsible.Content>
    </Collapsible.Root>
  );
}
