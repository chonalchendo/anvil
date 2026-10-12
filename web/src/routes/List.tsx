import { useParams, useSearch } from "@tanstack/react-router";
import { typeWord, useList } from "../lib/api";
import { FilterChip } from "../components/list/FilterChip";
import { ItemRows } from "../components/list/ItemRows";
import { Glyph } from "../components/ui/Glyph";
import { IssueRows } from "../components/ui/IssueRows";
import { PageHead } from "../components/ui/PageHead";
import { QueryStatus } from "../components/ui/QueryStatus";

// GitHub's view tabs: each status tab is a stored query, and the URL holds it.
export function ListPage() {
  const { type } = useParams({ from: "/t/$type" });
  const search = useSearch({ from: "/t/$type" });
  const { data, error } = useList(type, search);
  if (!data) return <QueryStatus error={error} what={`${typeWord(type)} list`} />;
  const statuses = Object.entries(data.counts).sort((a, b) => b[1] - a[1]);
  const all = Object.values(data.counts).reduce((a, b) => a + b, 0);
  const scope = `${search.project ? ` in ${search.project}` : ""}${search.topic ? ` on topic ${search.topic}` : ""}`;

  return (
    <div className="mx-auto max-w-[1080px] px-10 py-10">
      <PageHead title={<span className="capitalize">{typeWord(type)}s</span>} lede={`${data.total} shown${data.total >= 400 ? " (first 400)" : ""}${scope}.`} />
      <div className="mb-2 flex flex-wrap items-center gap-1" aria-label="Status">
        <FilterChip type={type} search={{ ...search, status: undefined }} isOn={!search.status}>
          All <span className="text-faint num">{all}</span>
        </FilterChip>
        {statuses.map(([s, n]) => (
          <FilterChip key={s} type={type} search={{ ...search, status: s }} isOn={search.status === s}>
            <Glyph status={s} size={12} />{s} <span className="text-faint num">{n}</span>
          </FilterChip>
        ))}
      </div>
      {data.projects.length > 1 && (
        <div className="mb-5 flex flex-wrap items-center gap-1 text-xs" aria-label="Project">
          <span className="mr-1 text-faint">Project</span>
          <FilterChip type={type} search={{ ...search, project: undefined }} isOn={!search.project}>any</FilterChip>
          {data.projects.map((p) => (
            <FilterChip key={p} type={type} search={{ ...search, project: p }} isOn={search.project === p}>{p}</FilterChip>
          ))}
        </div>
      )}
      <div className="rounded-lg border border-line bg-panel py-1">
        {type === "issue" ? <IssueRows rows={data.issues} showProject={!search.project} /> : <ItemRows rows={data.rows} />}
        {!data.rows.length && <div className="px-3 py-2 text-dim">Nothing matches.</div>}
      </div>
    </div>
  );
}
