import { useSearch } from "@tanstack/react-router";
import { useKnowledge } from "../lib/api";
import { KindTabs } from "../components/knowledge/KindTabs";
import { TopicList } from "../components/knowledge/TopicList";
import { Timeline } from "../components/Timeline";
import { PageHead } from "../components/ui/PageHead";
import { QueryStatus } from "../components/ui/QueryStatus";

export function KnowledgePage() {
  const { type = "" } = useSearch({ from: "/knowledge" });
  const { data, error } = useKnowledge(type);
  if (!data) return <QueryStatus error={error} what="knowledge" />;
  const c = data.counts;
  const lede = `${c.decision.accepted ?? 0} decisions accepted and ${c.decision.proposed ?? 0} proposed. ${c.learning.verified ?? 0} learnings verified, ${c.learning.draft ?? 0} still draft. ${c.thread.open ?? 0} threads open.`;

  return (
    <div className="mx-auto grid max-w-[1320px] grid-cols-[minmax(0,1fr)_280px] gap-14 px-10 py-10 max-xl:grid-cols-1">
      <div className="min-w-0">
        <PageHead title="Knowledge" lede={lede} />
        <KindTabs current={type} />
        <Timeline rows={data.rows} />
      </div>
      <TopicList topics={data.topics} />
    </div>
  );
}
