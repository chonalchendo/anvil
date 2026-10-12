import * as Collapsible from "@radix-ui/react-collapsible";
import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import { typeWord, type Stack } from "../lib/api";
import { cn } from "../lib/cn";
import { Prose } from "./Prose";
import { Glyph } from "./ui/Glyph";
import { TypeIcon } from "./ui/TypeIcon";

const kb = (b: number) => `${(b / 1024).toFixed(1)} KB`;

// The issue's hydrate set in load order. Each layer is a fold whose header
// carries its size, so the reader sees what the agent paid for before reading it.
export function StackLayers({ stack }: { stack: Stack }) {
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const total = stack.layers.reduce((s, l) => s + l.bytes, 0) || 1;
  const isAllOpen = stack.layers.every((l) => open[l.key]);
  return (
    <>
      <div className="mb-3 flex items-center justify-between text-xs text-dim">
        <span className="num">{stack.layers.length} layers · {kb(total)}, in the order the agent loads them</span>
        <button className="hover:text-fg" onClick={() => setOpen(Object.fromEntries(stack.layers.map((l) => [l.key, !isAllOpen])))}>
          {isAllOpen ? "Collapse all" : "Expand all"}
        </button>
      </div>
      <ol className="overflow-hidden rounded-lg border border-line bg-panel">
        {stack.layers.map((l, i) => (
          <Collapsible.Root key={l.key} asChild open={!!open[l.key]} onOpenChange={(o) => setOpen((s) => ({ ...s, [l.key]: o }))}>
            <li className="border-b border-line last:border-b-0">
              <Collapsible.Trigger className="grid h-11 w-full grid-cols-[16px_20px_16px_minmax(0,1fr)_auto_auto] items-center gap-2.5 px-3 text-left hover:bg-hover">
                <ChevronRight size={14} className={cn("text-faint transition-transform", open[l.key] && "rotate-90")} aria-hidden />
                <span className="text-xs text-faint num">{i + 1}</span>
                <TypeIcon type={l.type} />
                <span className="truncate text-sm text-fg">{l.title}</span>
                <span className="flex items-center gap-1.5 text-xs text-dim"><Glyph status={l.status} size={11} />{l.indexed ? typeWord(l.type) : "not indexed"}</span>
                <span className="flex items-center gap-2.5 text-xs text-dim num">
                  <span className="relative h-1 w-14 overflow-hidden rounded-full bg-line" aria-hidden>
                    <span className="absolute inset-y-0 left-0 rounded-full bg-dim" style={{ width: `${Math.max(3, (l.bytes / total) * 100)}%` }} />
                  </span>
                  <span className="w-16 whitespace-nowrap text-right">{kb(l.bytes)}</span>
                </span>
              </Collapsible.Trigger>
              <Collapsible.Content className="border-t border-line bg-bg px-12 py-6">
                {l.indexed ? (
                  <>
                    <Link to="/a/$key" params={{ key: l.key }} className="mb-4 inline-block font-mono text-xs text-faint hover:text-dim" translate="no">{l.key}</Link>
                    <Prose html={l.html} />
                  </>
                ) : (
                  <p className="text-sm text-dim">Hydrate loads <code translate="no">{l.key}</code>, but the index has no row for it. Run <code>anvil reindex</code>.</p>
                )}
              </Collapsible.Content>
            </li>
          </Collapsible.Root>
        ))}
      </ol>
      {stack.broken.length > 0 && (
        <section className="mt-4 rounded-lg border border-[color-mix(in_srgb,var(--color-s-warn)_40%,transparent)] px-4 py-3" aria-label="Broken edges">
          <p className="mb-1 flex items-center gap-2 text-sm text-fg"><Glyph status="escalated" size={12} /> {stack.broken.length} broken spine edge{stack.broken.length === 1 ? "" : "s"}; the walk stopped there.</p>
          <ul className="font-mono text-xs text-dim">{stack.broken.map((b) => <li key={b}>{b}</li>)}</ul>
        </section>
      )}
    </>
  );
}
