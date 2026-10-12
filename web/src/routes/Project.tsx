import * as Tabs from "@radix-ui/react-tabs";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { Maximize2 } from "lucide-react";
import { useProject, type IssueRow, type Project } from "../lib/api";
import { MilestoneGroup } from "../components/Milestone";
import { Board } from "../components/Board";
import { DiagramFrame } from "../components/DiagramFrame";
import { Caption, Empty, Glyph, IssueRows, Loading, Meter, PageHead, TypeIcon } from "../components/ui";

function Readout({ label, value, status, note }: { label: string; value: number; status?: string; note?: string }) {
  return (
    <div className="flex flex-col gap-1 border-l border-line pl-4 first:border-l-0 first:pl-0">
      <span className="text-xs text-dim">{label}</span>
      <span className="flex items-center gap-2 text-xl font-semibold text-fg num">
        {status && <Glyph status={status} size={12} />}
        {value}
      </span>
      {note && <span className="text-xs text-faint">{note}</span>}
    </div>
  );
}

const tab = "h-8 rounded-md px-3 text-sm text-dim hover:text-fg data-[state=active]:bg-raised data-[state=active]:text-fg";

export function ProjectPage() {
  const { slug } = useParams({ from: "/p/$slug" });
  const { data, error } = useProject(slug);
  const { view = "list" } = useSearch({ strict: false }) as { view?: string };
  const navigate = useNavigate();
  if (error) return <div className="p-10 text-dim">No project named {slug}.</div>;
  if (!data) return <Loading />;
  const c = data.counts.issues;
  const work: IssueRow[] = [...data.milestones.flatMap((m) => m.issues), ...data.loose];

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
            <Readout label="Milestones in flight" value={data.counts.milestones["in-progress"] ?? 0} note={`${data.counts.milestones.planned ?? 0} planned · ${data.counts.milestones.done ?? 0} done`} />
            <Readout label="Resolved" value={c.resolved ?? 0} status="resolved" />
          </div>
        </PageHead>

        <Tabs.Root value={view} onValueChange={(v) => navigate({ to: "/p/$slug", params: { slug }, search: (v === "list" ? {} : { view: v }) as any, replace: true })}>
          <div className="mb-3 flex items-center justify-between">
            <Caption>Open work</Caption>
            <Tabs.List aria-label="View" className="flex gap-1">
              <Tabs.Trigger value="list" className={tab}>By milestone</Tabs.Trigger>
              <Tabs.Trigger value="board" className={tab}>Board</Tabs.Trigger>
            </Tabs.List>
          </div>
          <Tabs.Content value="list">
            <Work data={data} />
          </Tabs.Content>
          <Tabs.Content value="board">
            <Board rows={work} />
          </Tabs.Content>
        </Tabs.Root>

        <section className="mt-12">
          <Caption className="mb-2">Recently done</Caption>
          <div className="rounded-lg border border-line">
            {data.done.map((m) => (
              <Link key={m.key} to="/a/$key" params={{ key: m.key }} className="flex h-10 items-center gap-2.5 border-b border-line px-3 last:border-b-0 hover:bg-hover">
                <Glyph status={m.status} />
                <span className="min-w-0 truncate text-sm text-body">{m.title}</span>
                <span className="ml-auto flex shrink-0 items-center gap-4 text-xs text-dim">
                  <Meter done={m.resolved} total={m.total} />
                  <span className="num">{m.done ?? m.updated}</span>
                </span>
              </Link>
            ))}
            {!data.done.length && <div className="p-3 text-dim">No milestone done yet.</div>}
          </div>
        </section>
      </div>

      <aside className="flex flex-col gap-8 pt-1">
        <Designs data={data} />
        <Side title="Decisions" rows={data.decisions} all={`/t/decision?project=${data.slug}`} />
        <Side title="Learnings" rows={data.learnings} all={`/t/learning?project=${data.slug}`} />
      </aside>
    </div>
  );
}

function Work({ data }: { data: Project }) {
  if (!data.milestones.length && !data.loose.length)
    return (
      <Empty>
        No milestone is open and no issue waits.{" "}
        <Link to="/a/$key" params={{ key: `product-design.${data.slug}` }} hash="milestones" className="text-fg underline">
          The product design lists the candidates.
        </Link>
      </Empty>
    );
  return (
    <div className="rounded-lg border border-line bg-panel">
      {data.milestones.map((m, i) => <MilestoneGroup key={m.key} m={m} defaultOpen={m.live || i === 0} />)}
      {data.loose.length > 0 && (
        <div className="border-t border-line py-2">
          <div className="px-3 pb-1 text-xs text-dim">Not under an open milestone</div>
          <IssueRows rows={data.loose} />
        </div>
      )}
    </div>
  );
}

function Designs({ data }: { data: Project }) {
  const diagrams = data.designs.flatMap((d) => d.diagrams.map((n) => ({ n, d })));
  return (
    <section>
      <Caption className="mb-2">Designs</Caption>
      {diagrams.length > 0 && (
        <div className="mb-3 flex flex-col gap-2">
          {diagrams.slice(0, 4).map(({ n }) => (
            <Link key={n} to="/diagram/$name" params={{ name: n }} className="group relative block overflow-hidden rounded-md border border-line bg-panel hover:border-line-strong">
              <div className="max-h-[168px] overflow-hidden"><DiagramFrame name={n} interactive={false} /></div>
              <span className="block truncate border-t border-line px-2 py-1 text-cap text-dim">{n}</span>
              <Maximize2 size={12} className="absolute right-1.5 top-1.5 text-faint opacity-0 group-hover:opacity-100" aria-hidden />
            </Link>
          ))}
        </div>
      )}
      <ul className="flex flex-col">
        {data.designs.slice(0, 10).map((d) => (
          <li key={d.key}>
            <Link to="/a/$key" params={{ key: d.key }} className="flex h-7 items-center gap-2 rounded-md px-1 text-sm text-body hover:bg-hover hover:text-fg">
              <TypeIcon type={d.type} />
              <span className="truncate">{d.title}</span>
            </Link>
          </li>
        ))}
        {data.designs.length > 10 && (
          <li><Link to="/t/$type" params={{ type: "component-design" }} search={{ project: data.slug } as any} className="px-1 text-xs text-faint hover:text-dim">{data.designs.length - 10} more component designs</Link></li>
        )}
      </ul>
    </section>
  );
}

function Side({ title, rows, all }: { title: string; rows: Project["decisions"]; all: string }) {
  return (
    <section>
      <Caption className="mb-2">{title}</Caption>
      <ul className="flex flex-col gap-3">
        {rows.map((r) => (
          <li key={r.key} className="flex gap-2">
            <Glyph status={r.status} size={12} className="mt-[4px]" />
            <div className="min-w-0">
              <Link to="/a/$key" params={{ key: r.key }} className="text-sm text-body hover:text-fg hover:underline line-clamp-2">{r.title}</Link>
              <div className="text-xs text-faint num">{r.status} · {r.updated}</div>
            </div>
          </li>
        ))}
        {!rows.length && <li className="text-sm text-dim">None yet.</li>}
      </ul>
      {rows.length > 0 && <a href={all} className="mt-2 inline-block text-xs text-dim hover:text-fg">All {title.toLowerCase()} →</a>}
    </section>
  );
}
