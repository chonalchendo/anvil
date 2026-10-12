import * as Collapsible from "@radix-ui/react-collapsible";
import { ChevronRight } from "lucide-react";
import type { Artifact } from "../../lib/api";

export function PropertyList({ props }: { props: Artifact["props"] }) {
  if (!props.length) return null;
  return (
    <Collapsible.Root>
      <Collapsible.Trigger className="group flex items-center gap-1 text-cap font-medium uppercase tracking-[0.04em] text-faint hover:text-dim">
        <ChevronRight size={12} className="group-data-[state=open]:rotate-90" aria-hidden /> All properties
      </Collapsible.Trigger>
      <Collapsible.Content>
        <dl className="mt-2 grid grid-cols-[110px_minmax(0,1fr)] gap-x-2 gap-y-1 text-xs">
          {props.map((p) => (
            <div key={p.k} className="contents">
              <dt className="truncate font-mono text-faint" translate="no">{p.k}</dt>
              <dd className="break-words text-dim">{p.v.join(", ")}</dd>
            </div>
          ))}
        </dl>
      </Collapsible.Content>
    </Collapsible.Root>
  );
}
