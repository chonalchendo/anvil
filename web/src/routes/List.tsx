import { Link, useParams, useSearch } from "@tanstack/react-router";
import { useList, age, typeWord, type IssueRow } from "../lib/api";
import { Glyph, IssueRows, Loading, PageHead } from "../components/ui";

const chip = "h-7 rounded-md px-2.5 text-sm text-dim hover:text-fg data-[on=true]:bg-raised data-[on=true]:text-fg";

// GitHub's view tabs: each status tab is a stored query, and the URL holds it.
export function ListPage() {
  const { type } = useParams({ from: "/t/$type" });
  const search = useSearch({ strict: false }) as { project?: string; status?: string; topic?: string };
  const params = new URLSearchParams(Object.entries(search).filter(([, v]) => v) as [string, string][]).toString();
  const { data, error } = useList(type, params);
  if (error) return <div className="p-10 text-dim">No artifacts of type {type}.</div>;
  if (!data) return <Loading />;
  const statuses = Object.entries(data.counts).sort((a, b) => b[1] - a[1]);
  const all = Object.values(data.counts).reduce((a, b) => a + b, 0);
  const to = (patch: Record<string, string | undefined>) => ({ ...search, ...patch });

  return (
    <div className="mx-auto max-w-[1080px] px-10 py-10">
      <PageHead title={<span className="capitalize">{typeWord(type)}s</span>} lede={`${data.total} shown${data.total >= 400 ? " (first 400)" : ""}${search.project ? ` in ${search.project}` : ""}${search.topic ? ` on topic ${search.topic}` : ""}.`} />
      <div className="mb-2 flex flex-wrap items-center gap-1" role="tablist" aria-label="Status">
        <Link to="/t/$type" params={{ type }} search={to({ status: undefined })} data-on={!search.status} className={chip}>
          All <span className="text-faint num">{all}</span>
        </Link>
        {statuses.map(([s, n]) => (
          <Link key={s} to="/t/$type" params={{ type }} search={to({ status: s })} data-on={search.status === s} className={`${chip} flex items-center gap-1.5`}>
            <Glyph status={s} size={12} />{s} <span className="text-faint num">{n}</span>
          </Link>
        ))}
      </div>
      {data.projects.length > 1 && (
        <div className="mb-5 flex flex-wrap items-center gap-1 text-xs" aria-label="Project">
          <span className="mr-1 text-faint">Project</span>
          <Link to="/t/$type" params={{ type }} search={to({ project: undefined })} data-on={!search.project} className={chip}>any</Link>
          {data.projects.map((p) => (
            <Link key={p} to="/t/$type" params={{ type }} search={to({ project: p })} data-on={search.project === p} className={chip}>{p}</Link>
          ))}
        </div>
      )}
      <div className="rounded-lg border border-line bg-panel py-1">
        {type === "issue" ? (
          <IssueRows rows={data.rows as IssueRow[]} showProject={!search.project} />
        ) : (
          data.rows.map((r) => (
            <Link key={r.key} to="/a/$key" params={{ key: r.key }} className="grid h-8 [content-visibility:auto] [contain-intrinsic-size:auto_32px] grid-cols-[16px_minmax(0,1fr)_auto_40px] items-center gap-2.5 rounded-md px-3 text-sm hover:bg-hover">
              <Glyph status={r.status} />
              <span className="truncate text-body">{r.title}</span>
              <span className="text-xs text-faint">{r.project}</span>
              <span className="text-right text-xs text-faint num" title={r.updated ?? ""}>{age(r.updated)}</span>
            </Link>
          ))
        )}
        {!data.rows.length && <div className="px-3 py-2 text-dim">Nothing matches.</div>}
      </div>
    </div>
  );
}
