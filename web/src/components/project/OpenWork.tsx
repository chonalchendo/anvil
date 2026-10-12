import { Link } from "@tanstack/react-router";
import type { Project } from "../../lib/api";
import { MilestoneGroup } from "../MilestoneGroup";
import { Empty } from "../ui/Empty";
import { IssueRows } from "../ui/IssueRows";

export function OpenWork({ project: p }: { project: Project }) {
  if (!p.milestones.length && !p.loose.length)
    return (
      <Empty>
        No milestone is open and no issue waits.{" "}
        <Link to="/a/$key" params={{ key: `product-design.${p.slug}` }} hash="milestones" className="text-fg underline">
          The product design lists the candidates.
        </Link>
      </Empty>
    );
  return (
    <div className="rounded-lg border border-line bg-panel">
      {p.milestones.map((m, i) => <MilestoneGroup key={m.key} milestone={m} defaultOpen={m.live || i === 0} />)}
      {p.loose.length > 0 && (
        <div className="border-t border-line py-2">
          <div className="px-3 pb-1 text-xs text-dim">Not under an open milestone</div>
          <IssueRows rows={p.loose} />
        </div>
      )}
    </div>
  );
}
