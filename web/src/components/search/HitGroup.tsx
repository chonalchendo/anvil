import { Link } from "@tanstack/react-router";
import { typeWord, type Hit } from "../../lib/api";
import { Caption } from "../ui/Caption";
import { Glyph } from "../ui/Glyph";
import { TypeIcon } from "../ui/TypeIcon";
import { Snippet } from "./Snippet";

export function HitGroup({ type, hits }: { type: string; hits: Hit[] }) {
  return (
    <section className="mb-8">
      <Caption className="mb-2 flex items-center gap-1.5"><TypeIcon type={type} />{typeWord(type)} · {hits.length}</Caption>
      <ul className="flex flex-col gap-1">
        {hits.map((h) => (
          <li key={h.key}>
            <Link to="/a/$key" params={{ key: h.key }} className="block rounded-md px-3 py-2.5 hover:bg-hover">
              <div className="flex items-center gap-2">
                <Glyph status={h.status} size={12} />
                <span className="truncate text-sm font-medium text-fg">{h.title}</span>
                <span className="ml-auto shrink-0 text-xs text-faint">{h.project}</span>
              </div>
              <p className="mt-1 line-clamp-2 pl-5 text-sm text-dim"><Snippet text={h.snippet} /></p>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
