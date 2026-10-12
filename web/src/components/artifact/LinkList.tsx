import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { typeWord, type Linked } from "../../lib/api";
import { Caption } from "../ui/Caption";
import { Glyph } from "../ui/Glyph";

const FOLD = 8;

// Quartz's backlinks panel: what cites this is as informative as what it cites.
export function LinkList({ title, rows }: { title: string; rows: Linked[] }) {
  const [showAll, setShowAll] = useState(false);
  if (!rows.length) return null;
  return (
    <section>
      <Caption className="mb-2">{title} · <span className="num">{rows.length}</span></Caption>
      <ul className="flex flex-col">
        {(showAll ? rows : rows.slice(0, FOLD)).map((r) => (
          <li key={r.key}>
            <Link to="/a/$key" params={{ key: r.key }} title={`${typeWord(r.type)} · ${r.relation}`} className="flex items-start gap-2 rounded-md px-1 py-1 text-sm leading-[18px] text-body hover:bg-hover hover:text-fg">
              <Glyph status={r.status} size={12} className="mt-[3px]" />
              <span className="line-clamp-2">{r.title}</span>
            </Link>
          </li>
        ))}
      </ul>
      {rows.length > FOLD && (
        <button onClick={() => setShowAll((s) => !s)} className="mt-1 px-1 text-xs text-faint hover:text-dim">
          {showAll ? "Show fewer" : `Show all ${rows.length}`}
        </button>
      )}
    </section>
  );
}
