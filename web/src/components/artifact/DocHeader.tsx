import { Link } from "@tanstack/react-router";
import { Layers } from "lucide-react";
import { typeWord, type Artifact } from "../../lib/api";
import { cn } from "../../lib/cn";
import { Glyph } from "../ui/Glyph";
import { Pill } from "../ui/Pill";
import { TypeIcon } from "../ui/TypeIcon";

const tab = "-mb-px flex items-center gap-1.5 border-b-2 border-transparent px-3 pb-2 text-sm text-dim hover:text-fg";
const tabOn = "border-fg text-fg";

export interface DocHeaderProps {
  artifact: Pick<Artifact, "key" | "type" | "title" | "status" | "project" | "updated" | "crumbs" | "description" | "tags">;
  /** An issue page shows Document and Context stack tabs; `current` marks which is open. */
  current?: "doc" | "stack";
}

// Stripe's head stack: breadcrumb, then type and key, then title and lede,
// then one state line.
export function DocHeader({ artifact: a, current }: DocHeaderProps) {
  return (
    <header className="mb-8">
      <nav aria-label="Breadcrumb" className="mb-4 flex min-w-0 items-center gap-1.5 text-xs text-dim">
        {a.crumbs.map((c, i) => (
          <span key={c.href} className="flex min-w-0 items-center gap-1.5">
            {i > 0 && <span aria-hidden>/</span>}
            <a href={c.href} className="truncate hover:text-fg">{c.label}</a>
          </span>
        ))}
      </nav>
      <div className="mb-2 flex items-center gap-1.5 text-xs text-dim">
        <TypeIcon type={a.type} /> <span className="capitalize">{typeWord(a.type)}</span>
        <span className="font-mono text-faint" translate="no">· {a.key.replace(`${a.type}.`, "")}</span>
      </div>
      <h1 className="max-w-[36em] text-xl font-semibold tracking-[-0.012em]">{a.title}</h1>
      {a.description && a.description !== a.title && <p className="mt-2 max-w-[68ch] text-base text-dim">{a.description}</p>}
      <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-dim">
        <span className="flex items-center gap-1.5"><Glyph status={a.status} size={12} />{a.status ?? "no status"}</span>
        {a.project && <Link to="/p/$slug" params={{ slug: a.project }} className="hover:text-fg">{a.project}</Link>}
        {a.updated && <span className="num">updated {a.updated}</span>}
        {a.tags.map((t) => <Pill key={t}>{t}</Pill>)}
      </div>
      {current && (
        <nav className="mt-6 flex gap-1 border-b border-line" aria-label="Issue views">
          <Link to="/a/$key" params={{ key: a.key }} aria-current={current === "doc" ? "page" : undefined} className={cn(tab, current === "doc" && tabOn)}>
            Document
          </Link>
          <Link to="/a/$key/stack" params={{ key: a.key }} aria-current={current === "stack" ? "page" : undefined} className={cn(tab, current === "stack" && tabOn)}>
            <Layers size={13} aria-hidden /> Context stack
          </Link>
        </nav>
      )}
    </header>
  );
}
