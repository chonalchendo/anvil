import { Link } from "@tanstack/react-router";
import type { Home } from "../../lib/api";
import { MilestoneGroup } from "../MilestoneGroup";
import { Timeline } from "../Timeline";
import { Caption } from "../ui/Caption";
import { Empty } from "../ui/Empty";
import { IssueRows } from "../ui/IssueRows";
import { PageHead } from "../ui/PageHead";
import { ProjectCard } from "./ProjectCard";

const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? "" : "s"}`;

export function HomeView({ home }: { home: Home }) {
  const live = home.milestones.reduce((a, m) => a + (m.counts["in-progress"] ?? 0), 0) + home.loose.filter((i) => i.status === "in-progress").length;
  const moving = new Set([...home.milestones.filter((m) => m.live).map((m) => m.project), ...home.loose.map((i) => i.project)]).size;
  return (
    <div className="mx-auto max-w-[1180px] px-10 py-10">
      <PageHead title="Now" lede={`${plural(live, "issue")} in progress across ${plural(moving, "project")}; ${plural(home.milestones.length, "milestone")} in flight.`} />
      <section aria-labelledby="inflight" className="mb-12">
        <Caption className="mb-2"><span id="inflight">In flight</span></Caption>
        {home.milestones.length || home.loose.length ? (
          <div className="rounded-lg border border-line bg-panel">
            {home.milestones.map((m) => <MilestoneGroup key={m.key} milestone={m} showProject defaultOpen={m.live} />)}
            {home.loose.length > 0 && (
              <div className="py-2">
                <div className="px-3 pb-1 text-xs text-dim">In progress or escalated outside an in-flight milestone</div>
                <IssueRows rows={home.loose} showProject />
              </div>
            )}
          </div>
        ) : (
          <Empty>Nothing is in flight. Open a project to see what is planned next.</Empty>
        )}
      </section>
      <section aria-labelledby="projects" className="mb-12">
        <Caption className="mb-2"><span id="projects">Projects</span></Caption>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
          {home.projects.map((p) => <ProjectCard key={p.slug} project={p} />)}
        </div>
      </section>
      <section aria-labelledby="recent">
        <Caption className="mb-2"><span id="recent">Decided and learned lately</span></Caption>
        <Timeline rows={home.recent} />
        <Link to="/knowledge" className="mt-3 inline-block text-xs text-dim hover:text-fg">All knowledge →</Link>
      </section>
    </div>
  );
}
