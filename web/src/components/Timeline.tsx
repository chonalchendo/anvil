import { Link } from "@tanstack/react-router";
import type { Described } from "../lib/api";
import { typeWord } from "../lib/api";
import { Glyph, Pill, TypeIcon } from "./ui";

const fmt = (d: string) => new Date(d).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

// Linear's changelog: the date sits sticky in a gutter while a day's entries
// scroll past, so time stays anchored without a date on every row.
export function Timeline({ rows }: { rows: Described[] }) {
  const days = new Map<string, Described[]>();
  for (const r of rows) {
    const d = (r.updated ?? r.created ?? "").slice(0, 10) || "undated";
    (days.get(d) ?? days.set(d, []).get(d)!).push(r);
  }
  return (
    <div>
      {[...days.entries()].map(([day, entries]) => (
        <section key={day} className="[content-visibility:auto] [contain-intrinsic-size:auto_160px] grid grid-cols-[120px_minmax(0,1fr)] gap-6 border-t border-line py-4 first:border-t-0">
          <div className="relative">
            <div className="sticky top-4 text-xs text-dim num">{day === "undated" ? day : fmt(day)}</div>
          </div>
          <ul className="flex flex-col gap-4">
            {entries.map((e) => (
              <li key={e.key} className="max-w-[72ch]">
                <div className="flex items-center gap-2">
                  <TypeIcon type={e.type} />
                  <Link to="/a/$key" params={{ key: e.key }} className="min-w-0 text-[14px] font-medium text-fg hover:underline">
                    {e.title}
                  </Link>
                </div>
                {e.description && <p className="mt-1 pl-[22px] text-sm text-dim line-clamp-2">{e.description}</p>}
                <div className="mt-1.5 flex items-center gap-2 pl-[22px] text-xs text-faint">
                  <Glyph status={e.status} size={11} />
                  <span>{typeWord(e.type)} · {e.status}</span>
                  {e.confidence && <Pill>{e.confidence} confidence</Pill>}
                  {e.project && <span>· {e.project}</span>}
                  {e.topic && <span>· {e.topic}</span>}
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
