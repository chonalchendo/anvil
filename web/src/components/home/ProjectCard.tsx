import { Link } from "@tanstack/react-router";
import { age, type ProjectSummary } from "../../lib/api";
import { Glyph } from "../ui/Glyph";

// GitHub's card anatomy: identity, then a two-line promise, then the numbers.
export function ProjectCard({ project: p }: { project: ProjectSummary }) {
  return (
    <Link to="/p/$slug" params={{ slug: p.slug }} className="group flex flex-col gap-2 rounded-lg border border-line bg-panel p-4 hover:border-line-strong hover:bg-raised">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-semibold text-fg">{p.slug}</span>
        <span className="text-xs text-faint num" title={p.updated}>{age(p.updated)}</span>
      </div>
      <p className="line-clamp-2 min-h-[39px] text-sm text-dim">{p.description ?? "No product design yet."}</p>
      <div className="flex gap-4 text-xs text-dim num">
        <span className="flex items-center gap-1"><Glyph status="in-progress" size={11} />{p.inProgress}</span>
        <span className="flex items-center gap-1"><Glyph status="open" size={11} />{p.open} open</span>
        <span>{p.milestonesInFlight} in flight · {p.milestonesPlanned} planned</span>
      </div>
    </Link>
  );
}
