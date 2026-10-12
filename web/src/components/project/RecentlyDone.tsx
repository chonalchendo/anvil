import { Link } from "@tanstack/react-router";
import type { Milestone } from "../../lib/api";
import { Caption } from "../ui/Caption";
import { Glyph } from "../ui/Glyph";
import { Meter } from "../ui/Meter";

export function RecentlyDone({ milestones }: { milestones: Milestone[] }) {
  return (
    <section className="mt-12">
      <Caption className="mb-2">Recently done</Caption>
      <div className="rounded-lg border border-line">
        {milestones.map((m) => (
          <Link key={m.key} to="/a/$key" params={{ key: m.key }} className="flex h-10 items-center gap-2.5 border-b border-line px-3 last:border-b-0 hover:bg-hover">
            <Glyph status={m.status} />
            <span className="min-w-0 truncate text-sm text-body">{m.title}</span>
            <span className="ml-auto flex shrink-0 items-center gap-4 text-xs text-dim">
              <Meter done={m.resolved} total={m.total} />
              <span className="num">{m.done ?? m.updated}</span>
            </span>
          </Link>
        ))}
        {!milestones.length && <div className="p-3 text-dim">No milestone done yet.</div>}
      </div>
    </section>
  );
}
