import { Link } from "@tanstack/react-router";
import { Maximize2 } from "lucide-react";
import type { Project } from "../../lib/api";
import { DiagramFrame } from "../DiagramFrame";
import { Caption } from "../ui/Caption";
import { TypeIcon } from "../ui/TypeIcon";

const SHOWN = 10;

export function Designs({ project: p }: { project: Project }) {
  const diagrams = p.designs.flatMap((d) => d.diagrams);
  return (
    <section>
      <Caption className="mb-2">Designs</Caption>
      {diagrams.length > 0 && (
        <div className="mb-3 flex flex-col gap-2">
          {diagrams.slice(0, 4).map((n) => (
            <Link key={n} to="/diagram/$name" params={{ name: n }} className="group relative block overflow-hidden rounded-md border border-line bg-panel hover:border-line-strong">
              <div className="max-h-[168px] overflow-hidden"><DiagramFrame name={n} interactive={false} /></div>
              <span className="block truncate border-t border-line px-2 py-1 text-cap text-dim">{n}</span>
              <Maximize2 size={12} className="absolute right-1.5 top-1.5 text-faint opacity-0 group-hover:opacity-100" aria-hidden />
            </Link>
          ))}
        </div>
      )}
      <ul className="flex flex-col">
        {p.designs.slice(0, SHOWN).map((d) => (
          <li key={d.key}>
            <Link to="/a/$key" params={{ key: d.key }} className="flex h-7 items-center gap-2 rounded-md px-1 text-sm text-body hover:bg-hover hover:text-fg">
              <TypeIcon type={d.type} />
              <span className="truncate">{d.title}</span>
            </Link>
          </li>
        ))}
        {p.designs.length > SHOWN && (
          <li>
            <Link to="/t/$type" params={{ type: "component-design" }} search={{ project: p.slug }} className="px-1 text-xs text-faint hover:text-dim">
              {p.designs.length - SHOWN} more component designs
            </Link>
          </li>
        )}
      </ul>
    </section>
  );
}
