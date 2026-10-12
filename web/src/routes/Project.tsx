import * as Tabs from "@radix-ui/react-tabs";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useProject } from "../lib/api";
import { Board } from "../components/Board";
import { Designs } from "../components/project/Designs";
import { OpenWork } from "../components/project/OpenWork";
import { Readout } from "../components/project/Readout";
import { RecentlyDone } from "../components/project/RecentlyDone";
import { SideList } from "../components/project/SideList";
import { Caption } from "../components/ui/Caption";
import { PageHead } from "../components/ui/PageHead";
import { QueryStatus } from "../components/ui/QueryStatus";

const tab = "h-8 rounded-md px-3 text-sm text-dim hover:text-fg data-[state=active]:bg-raised data-[state=active]:text-fg";

export function ProjectPage() {
  const { slug } = useParams({ from: "/p/$slug" });
  const { view = "list" } = useSearch({ from: "/p/$slug" });
  const navigate = useNavigate();
  const { data, error } = useProject(slug);
  if (!data) return <QueryStatus error={error} what="project" />;
  const c = data.counts.issues;
  const m = data.counts.milestones;

  return (
    <div className="mx-auto grid max-w-[1400px] grid-cols-[minmax(0,1fr)_320px] gap-12 px-10 py-10 max-xl:grid-cols-1">
      <div className="min-w-0">
        <PageHead
          crumbs={<><Link to="/" className="hover:text-fg">Now</Link><span aria-hidden>/</span><span>Projects</span></>}
          title={data.slug}
          lede={data.description}
        >
          <div className="mt-6 flex gap-6">
            <Readout label="In progress" value={c["in-progress"] ?? 0} status="in-progress" />
            <Readout label="Open" value={c.open ?? 0} status="open" />
            <Readout label="Escalated" value={c.escalated ?? 0} status="escalated" />
            <Readout label="Milestones in flight" value={m["in-progress"] ?? 0} note={`${m.planned ?? 0} planned · ${m.done ?? 0} done`} />
            <Readout label="Resolved" value={c.resolved ?? 0} status="resolved" />
          </div>
        </PageHead>

        <Tabs.Root value={view} onValueChange={(v) => navigate({ to: "/p/$slug", params: { slug }, search: v === "board" ? { view: "board" } : {}, replace: true })}>
          <div className="mb-3 flex items-center justify-between">
            <Caption>Open work</Caption>
            <Tabs.List aria-label="View" className="flex gap-1">
              <Tabs.Trigger value="list" className={tab}>By milestone</Tabs.Trigger>
              <Tabs.Trigger value="board" className={tab}>Board</Tabs.Trigger>
            </Tabs.List>
          </div>
          <Tabs.Content value="list"><OpenWork project={data} /></Tabs.Content>
          <Tabs.Content value="board"><Board rows={[...data.milestones.flatMap((x) => x.issues), ...data.loose]} /></Tabs.Content>
        </Tabs.Root>

        <RecentlyDone milestones={data.done} />
      </div>

      <aside className="flex flex-col gap-8 pt-1">
        <Designs project={data} />
        <SideList title="Decisions" rows={data.decisions} type="decision" project={data.slug} />
        <SideList title="Learnings" rows={data.learnings} type="learning" project={data.slug} />
      </aside>
    </div>
  );
}
