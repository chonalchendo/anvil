import * as Collapsible from "@radix-ui/react-collapsible";
import { Link, useParams } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import { useArtifact, useStack, typeWord } from "../lib/api";
import { DocHeader } from "./Artifact";
import { Glyph, Loading, TypeIcon } from "../components/ui";

const kb = (b: number) => `${(b / 1024).toFixed(1)} KB`;

// The issue's hydrate set in load order. Each layer is a fold whose header
// carries its size, so the reader sees what the agent paid for before reading it.
export function StackPage() {
  const { key } = useParams({ from: "/a/$key/stack" });
  const { data: a } = useArtifact(key);
  const { data } = useStack(key);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  if (!a || !data) return <Loading />;
  const total = data.layers.reduce((s, l) => s + l.bytes, 0);
  const allOpen = data.layers.every((l) => open[l.key]);

  return (
    <div className="mx-auto max-w-[960px] px-10 py-10">
      <DocHeader a={a} tabs="stack" />
      <div className="mb-3 flex items-center justify-between text-xs text-dim">
        <span className="num">{data.layers.length} layers · {kb(total)}, in the order the agent loads them</span>
        <button
          className="hover:text-fg"
          onClick={() => setOpen(Object.fromEntries(data.layers.map((l) => [l.key, !allOpen])))}
        >
          {allOpen ? "Collapse all" : "Expand all"}
        </button>
      </div>
      <ol className="overflow-hidden rounded-lg border border-line bg-panel">
        {data.layers.map((l, i) => (
          <Collapsible.Root key={l.key} asChild open={!!open[l.key]} onOpenChange={(o) => setOpen((s) => ({ ...s, [l.key]: o }))}>
            <li className="border-b border-line last:border-b-0">
              <Collapsible.Trigger className="grid h-11 w-full grid-cols-[16px_20px_16px_minmax(0,1fr)_auto_auto] items-center gap-2.5 px-3 text-left hover:bg-hover">
                <ChevronRight size={14} className={`text-faint transition-transform ${open[l.key] ? "rotate-90" : ""}`} aria-hidden />
                <span className="text-xs text-faint num">{i + 1}</span>
                <TypeIcon type={l.type} />
                <span className="truncate text-sm text-fg">{l.title}</span>
                <span className="flex items-center gap-1.5 text-xs text-dim"><Glyph status={l.status} size={11} />{typeWord(l.type)}</span>
                <span className="flex items-center gap-2.5 text-xs text-dim num">
                  <span className="relative h-1 w-14 overflow-hidden rounded-full bg-line" aria-hidden>
                    <span className="absolute inset-y-0 left-0 rounded-full bg-dim" style={{ width: `${Math.max(3, (l.bytes / total) * 100)}%` }} />
                  </span>
                  <span className="w-14 text-right">{kb(l.bytes)}</span>
                </span>
              </Collapsible.Trigger>
              <Collapsible.Content className="border-t border-line bg-bg px-12 py-6">
                <Link to="/a/$key" params={{ key: l.key }} className="mb-4 inline-block font-mono text-xs text-faint hover:text-dim" translate="no">{l.key}</Link>
                <div className="prose" dangerouslySetInnerHTML={{ __html: l.html }} />
              </Collapsible.Content>
            </li>
          </Collapsible.Root>
        ))}
      </ol>
    </div>
  );
}
