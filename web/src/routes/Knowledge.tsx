import { Link, useSearch } from "@tanstack/react-router";
import { useKnowledge } from "../lib/api";
import { Timeline } from "../components/Timeline";
import { Caption, Loading, PageHead } from "../components/ui";

const tabs = [
  { v: "", label: "Everything" },
  { v: "decision", label: "Decisions" },
  { v: "learning", label: "Learnings" },
  { v: "thread", label: "Threads" },
];

export function KnowledgePage() {
  const { type = "" } = useSearch({ strict: false }) as { type?: string };
  const { data } = useKnowledge(type);
  if (!data) return <Loading />;
  const c = data.counts;
  const lede = `${c.decision.accepted ?? 0} decisions accepted and ${c.decision.proposed ?? 0} proposed. ${c.learning.verified ?? 0} learnings verified, ${c.learning.draft ?? 0} still draft. ${c.thread.open ?? 0} threads open.`;

  return (
    <div className="mx-auto grid max-w-[1320px] grid-cols-[minmax(0,1fr)_280px] gap-14 px-10 py-10 max-xl:grid-cols-1">
      <div className="min-w-0">
        <PageHead title="Knowledge" lede={lede} />
        <div className="mb-4 flex gap-1" role="tablist" aria-label="Kind">
          {tabs.map((t) => (
            <Link
              key={t.v} to="/knowledge" search={(t.v ? { type: t.v } : {}) as any} data-on={type === t.v}
              className="h-7 rounded-md px-2.5 text-sm leading-7 inline-block text-dim hover:text-fg data-[on=true]:bg-raised data-[on=true]:text-fg"
            >
              {t.label}
            </Link>
          ))}
        </div>
        <Timeline rows={data.rows} />
      </div>
      <aside className="pt-1">
        <Caption className="mb-2">Topics</Caption>
        <ul className="flex flex-col">
          {data.topics.slice(0, 40).map((t) => (
            <li key={t.slug}>
              <Link
                to="/t/$type" params={{ type: t.decisions ? "decision" : "thread" }} search={{ topic: t.slug } as any}
                className="flex h-8 items-center gap-2 rounded-md px-2 text-sm text-body hover:bg-hover hover:text-fg"
                title={`last moved ${t.moved}`}
              >
                <span className="truncate" translate="no">{t.slug}</span>
                <span className="ml-auto shrink-0 text-xs text-faint num">
                  {t.decisions ? `${t.decisions} dec` : ""}{t.decisions && t.threads ? " · " : ""}{t.threads ? `${t.threads} thr` : ""}{t.open ? ` · ${t.open} open` : ""}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </aside>
    </div>
  );
}
