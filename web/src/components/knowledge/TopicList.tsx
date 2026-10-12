import { Link } from "@tanstack/react-router";
import type { Knowledge } from "../../lib/api";
import { Caption } from "../ui/Caption";

const SHOWN = 40;

function tally(t: Knowledge["topics"][number]) {
  return [t.decisions && `${t.decisions} dec`, t.threads && `${t.threads} thr`, t.open && `${t.open} open`].filter(Boolean).join(" · ");
}

export function TopicList({ topics }: { topics: Knowledge["topics"] }) {
  return (
    <aside className="pt-1">
      <Caption className="mb-2">Topics</Caption>
      <ul className="flex flex-col">
        {topics.slice(0, SHOWN).map((t) => (
          <li key={t.slug}>
            <Link
              to="/t/$type" params={{ type: t.decisions ? "decision" : "thread" }} search={{ topic: t.slug }}
              className="flex h-8 items-center gap-2 rounded-md px-2 text-sm text-body hover:bg-hover hover:text-fg"
              title={`last moved ${t.moved}`}
            >
              <span className="truncate" translate="no">{t.slug}</span>
              <span className="ml-auto shrink-0 text-xs text-faint num">{tally(t)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </aside>
  );
}
