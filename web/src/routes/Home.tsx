import { Link } from "@tanstack/react-router";
import { useHome, age } from "../lib/api";
import { MilestoneGroup } from "../components/Milestone";
import { Timeline } from "../components/Timeline";
import { Caption, Empty, Glyph, IssueRows, Loading, PageHead } from "../components/ui";

const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? "" : "s"}`;

export function HomePage() {
  const { data } = useHome();
  if (!data) return <Loading />;
  const live = data.milestones.reduce((a, m) => a + (m.counts["in-progress"] ?? 0), 0) + data.loose.length;
  const projectsMoving = new Set([...data.milestones.filter((m) => m.live).map((m) => m.project), ...data.loose.map((i) => i.project)]).size;

  return (
    <div className="mx-auto max-w-[1180px] px-10 py-10">
      <PageHead
        title="Now"
        lede={`${plural(live, "issue")} in progress across ${plural(projectsMoving, "project")}; ${plural(data.milestones.length, "milestone")} in flight.`}
      />

      <section aria-labelledby="inflight" className="mb-12">
        <Caption className="mb-2"><span id="inflight">In flight</span></Caption>
        {data.milestones.length || data.loose.length ? (
          <div className="rounded-lg border border-line bg-panel">
            {data.milestones.map((m) => <MilestoneGroup key={m.key} m={m} showProject defaultOpen={m.live} />)}
            {data.loose.length > 0 && (
              <div className="py-2">
                <div className="px-3 pb-1 text-xs text-dim">In progress outside an in-flight milestone</div>
                <IssueRows rows={data.loose} showProject />
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
          {data.projects.map((p) => (
            <Link
              key={p.slug} to="/p/$slug" params={{ slug: p.slug }}
              className="group flex flex-col gap-2 rounded-lg border border-line bg-panel p-4 hover:border-line-strong hover:bg-raised"
            >
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
          ))}
        </div>
      </section>

      <section aria-labelledby="recent">
        <Caption className="mb-2"><span id="recent">Decided and learned lately</span></Caption>
        <Timeline rows={data.recent} />
        <Link to="/knowledge" className="mt-3 inline-block text-xs text-dim hover:text-fg">All knowledge →</Link>
      </section>
    </div>
  );
}
