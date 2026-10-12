import { Command } from "cmdk";
import * as Dialog from "@radix-ui/react-dialog";
import { useNavigate } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { typeWord, usePalette } from "../lib/api";
import { cn } from "../lib/cn";
import { group, rank } from "../lib/rank";
import { Glyph } from "./ui/Glyph";
import { TypeIcon } from "./ui/TypeIcon";

const heading =
  "[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-cap [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-[0.04em] [&_[cmdk-group-heading]]:text-faint";
const row = "flex h-9 cursor-pointer items-center gap-2.5 rounded-md px-2 text-sm data-[selected=true]:bg-hover data-[selected=true]:text-fg";

export interface PaletteProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}

// shadcn/cmdk: focus stays in the input while the active row moves, so typing
// and choosing never fight for focus. The last row falls through to body search.
export function Palette({ isOpen, onOpenChange }: PaletteProps) {
  const [term, setTerm] = useState("");
  const { data = [], error } = usePalette(isOpen);
  const navigate = useNavigate();
  const groups = useMemo(() => group(rank(data, term), term), [data, term]);

  useEffect(() => { if (!isOpen) setTerm(""); }, [isOpen]);

  const handleSelect = (href: string) => { onOpenChange(false); navigate({ to: href }); };

  return (
    <Dialog.Root open={isOpen} onOpenChange={onOpenChange}>
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
              {error && <div role="alert" className="px-2 py-3 text-sm text-dim">Could not load the index: {error.message}.</div>}
              {!data.length && !error && <Command.Loading>Loading…</Command.Loading>}
              {groups.map(([type, entries]) => (
                <Command.Group key={type} heading={typeWord(type)} className={heading}>
                  {entries.map((e) => (
                    <Command.Item key={e.href} value={e.href} onSelect={() => handleSelect(e.href)} className={cn(row, "text-body")}>
                      {e.status ? <Glyph status={e.status} /> : <TypeIcon type={e.type} />}
                      <span className="truncate">{e.title}</span>
                      {e.project && e.type !== "project" && <span className="ml-auto shrink-0 text-xs text-faint">{e.project}</span>}
                    </Command.Item>
                  ))}
                </Command.Group>
              ))}
              {term.trim() && (
                <Command.Item value={`search:${term}`} onSelect={() => handleSelect(`/search?q=${encodeURIComponent(term.trim())}`)} className={cn(row, "mt-1 border-t border-line text-dim")}>
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
