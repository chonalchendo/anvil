import * as Collapsible from "@radix-ui/react-collapsible";
import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import type { Milestone } from "../lib/api";
import { Glyph, IssueRows, Meter, Pill } from "./ui";

const rank: Record<string, number> = { "in-progress": 0, escalated: 1, open: 2, resolved: 3, abandoned: 4 };

// A milestone fold: the header is its own readout (status, meter, measured),
// the body lists live work first and keeps finished work one click away.
export function MilestoneGroup({ m, defaultOpen, showProject }: { m: Milestone; defaultOpen?: boolean; showProject?: boolean }) {
  const [open, setOpen] = useState(defaultOpen ?? m.live);
  const [showDone, setShowDone] = useState(false);
  const sorted = [...m.issues].sort((a, b) => (rank[a.status ?? ""] ?? 9) - (rank[b.status ?? ""] ?? 9));
  const active = sorted.filter((i) => i.status !== "resolved" && i.status !== "abandoned");
  const finished = sorted.length - active.length;
  return (
    <Collapsible.Root open={open} onOpenChange={setOpen} className="border-b border-line last:border-b-0">
      <div className="flex h-10 items-center gap-2.5 px-3">
        <Collapsible.Trigger aria-label={open ? "Collapse" : "Expand"} className="-ml-1 grid size-5 place-items-center rounded-sm text-faint hover:bg-hover hover:text-fg">
          <ChevronRight size={14} className={open ? "rotate-90 transition-transform" : "transition-transform"} />
        </Collapsible.Trigger>
        <Glyph status={m.status} />
        <Link to="/a/$key" params={{ key: m.key }} className="min-w-0 truncate text-sm font-medium text-fg hover:underline">{m.title}</Link>
        {showProject && m.project && <Pill>{m.project}</Pill>}
        <span className="ml-auto flex shrink-0 items-center gap-4 text-xs text-dim">
          {m.counts["in-progress"] ? <span className="flex items-center gap-1 num"><Glyph status="in-progress" size={11} />{m.counts["in-progress"]}</span> : null}
          {m.counts.escalated ? <span className="flex items-center gap-1 num"><Glyph status="escalated" size={11} />{m.counts.escalated}</span> : null}
          {m.counts.open ? <span className="flex items-center gap-1 num"><Glyph status="open" size={11} />{m.counts.open}</span> : null}
          <Meter done={m.resolved} total={m.total} live={m.live} />
          <span className={m.measured ? "text-dim" : "text-faint"} title={m.measured ?? "No Measured: line yet"}>
            {m.measured ? `measured ${m.measured.slice(0, 10)}` : "not measured"}
          </span>
        </span>
      </div>
      <Collapsible.Content className="pb-2 pl-5">
        <IssueRows rows={showDone ? sorted : active} empty={finished ? "No open issues." : "No issues yet."} />
        {finished > 0 && (
          <button onClick={() => setShowDone((s) => !s)} className="ml-3 mt-1 text-xs text-faint hover:text-dim">
            {showDone ? "Hide" : "Show"} {finished} finished
          </button>
        )}
      </Collapsible.Content>
    </Collapsible.Root>
  );
}
