import { Link } from "@tanstack/react-router";
import type { Described } from "../../lib/api";
import { Caption } from "../ui/Caption";
import { Glyph } from "../ui/Glyph";

export interface SideListProps {
  title: string;
  rows: Described[];
  type: "decision" | "learning";
  project: string;
}

export function SideList({ title, rows, type, project }: SideListProps) {
  return (
    <section>
      <Caption className="mb-2">{title}</Caption>
      <ul className="flex flex-col gap-3">
        {rows.map((r) => (
          <li key={r.key} className="flex gap-2">
            <Glyph status={r.status} size={12} className="mt-[4px]" />
            <div className="min-w-0">
              <Link to="/a/$key" params={{ key: r.key }} className="line-clamp-2 text-sm text-body hover:text-fg hover:underline">{r.title}</Link>
              <div className="text-xs text-faint num">{r.status} · {r.updated}</div>
            </div>
          </li>
        ))}
        {!rows.length && <li className="text-sm text-dim">None yet.</li>}
      </ul>
      {rows.length > 0 && (
        <Link to="/t/$type" params={{ type }} search={{ project }} className="mt-2 inline-block text-xs text-dim hover:text-fg">
          All {title.toLowerCase()} →
        </Link>
      )}
    </section>
  );
}
