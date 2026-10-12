import { Command } from "cmdk";
import * as Dialog from "@radix-ui/react-dialog";
import { useNavigate } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { usePalette, typeWord, type Entry } from "../lib/api";
import { Glyph, TypeIcon } from "./ui";

const order = ["project", "milestone", "issue", "product-design", "system-design", "component-design", "decision", "thread", "learning", "convention", "inbox"];

// Exact title or key prefix beats a word prefix beats a substring; within a
// tier, type weight then recency (the server sends entries in that order).
function rank(entries: Entry[], term: string) {
  const t = term.toLowerCase().trim();
  if (!t) return entries.filter((e) => ["project", "milestone"].includes(e.type) || e.status === "in-progress").slice(0, 40);
  const words = t.split(/\s+/);
  const scored: [number, number, Entry][] = [];
  entries.forEach((e, i) => {
    const title = e.title.toLowerCase();
    const key = e.key.toLowerCase();
    if (!words.every((w) => title.includes(w) || key.includes(w))) return;
    const tier = title.startsWith(t) || key.split(".").some((p) => p.startsWith(t)) ? 0
      : new RegExp(`\\b${t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`).test(title) ? 1 : 2;
    scored.push([tier, i, e]);
  });
  return scored.sort((a, b) => a[0] - b[0] || a[1] - b[1]).slice(0, 60).map((s) => s[2]);
}

export function Palette({ open, setOpen }: { open: boolean; setOpen: (o: boolean) => void }) {
  const [term, setTerm] = useState("");
  const { data = [] } = usePalette(open);
  const navigate = useNavigate();
  const hits = useMemo(() => rank(data, term), [data, term]);
  const groups = useMemo(() => {
    const g = new Map<string, Entry[]>();
    for (const h of hits) (g.get(h.type) ?? g.set(h.type, []).get(h.type)!).push(h);
    // Map keeps insertion order, so a group sits where its best hit ranked.
    return term.trim() ? [...g.entries()] : [...g.entries()].sort((a, b) => order.indexOf(a[0]) - order.indexOf(b[0]));
  }, [hits]);

  useEffect(() => { if (!open) setTerm(""); }, [open]);

  const go = (href: string) => { setOpen(false); navigate({ to: href }); };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content
          aria-describedby={undefined}
          className="fixed left-1/2 top-[12vh] z-50 w-[640px] max-w-[92vw] -translate-x-1/2 overflow-hidden rounded-lg border border-line-strong bg-panel shadow-[0_16px_48px_rgba(0,0,0,0.5)]"
        >
          <Dialog.Title className="sr-only">Go to anything</Dialog.Title>
          <Command shouldFilter={false} loop label="Go to anything">
            <div className="flex items-center gap-2 border-b border-line px-4">
              <Search size={15} className="text-faint" aria-hidden />
              <Command.Input
                value={term} onValueChange={setTerm} autoFocus name="palette" autoComplete="off" spellCheck={false}
                placeholder="Go to a project, milestone, issue, decision…"
                className="h-12 flex-1 bg-transparent text-base text-fg outline-none placeholder:text-faint"
              />
              <kbd className="rounded-sm border border-line-strong px-1.5 text-cap text-faint">esc</kbd>
            </div>
            <Command.List aria-live="polite" className="max-h-[56vh] overflow-y-auto overscroll-contain p-2">
              {!data.length && <Command.Loading>Loading…</Command.Loading>}
              {groups.map(([type, entries]) => (
                <Command.Group
                  key={type} heading={typeWord(type)}
                  className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-cap [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-[0.04em] [&_[cmdk-group-heading]]:text-faint"
                >
                  {entries.map((e) => (
                    <Command.Item
                      key={e.href} value={e.href} onSelect={() => go(e.href)}
                      className="flex h-9 cursor-pointer items-center gap-2.5 rounded-md px-2 text-sm text-body data-[selected=true]:bg-hover data-[selected=true]:text-fg"
                    >
                      {e.status ? <Glyph status={e.status} /> : <TypeIcon type={e.type} />}
                      <span className="truncate">{e.title}</span>
                      {e.project && e.type !== "project" && <span className="ml-auto shrink-0 text-xs text-faint">{e.project}</span>}
                    </Command.Item>
                  ))}
                </Command.Group>
              ))}
              {term.trim() && (
                <Command.Item
                  value={`search:${term}`} onSelect={() => go(`/search?q=${encodeURIComponent(term.trim())}`)}
                  className="mt-1 flex h-9 cursor-pointer items-center gap-2.5 rounded-md border-t border-line px-2 text-sm text-dim data-[selected=true]:bg-hover data-[selected=true]:text-fg"
                >
                  <Search size={14} aria-hidden /> Search every body for “{term.trim()}”
                </Command.Item>
              )}
            </Command.List>
          </Command>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
