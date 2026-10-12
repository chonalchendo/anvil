import { Link } from "@tanstack/react-router";
import { age, type Item } from "../../lib/api";
import { Glyph } from "../ui/Glyph";

export function ItemRows({ rows }: { rows: Item[] }) {
  return rows.map((r) => (
    <Link
      key={r.key} to="/a/$key" params={{ key: r.key }}
      className="grid h-8 grid-cols-[16px_minmax(0,1fr)_auto_40px] items-center gap-2.5 rounded-md px-3 text-sm [contain-intrinsic-size:auto_32px] [content-visibility:auto] hover:bg-hover"
    >
      <Glyph status={r.status} />
      <span className="truncate text-body">{r.title}</span>
      <span className="text-xs text-faint">{r.project}</span>
      <span className="text-right text-xs text-faint num" title={r.updated ?? ""}>{age(r.updated)}</span>
    </Link>
  ));
}
