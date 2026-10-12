import { Link, Outlet } from "@tanstack/react-router";
import { BookOpen, Home, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { useNav } from "../lib/api";
import { NavLink, navItem } from "./NavLink";
import { Palette } from "./Palette";
import { Caption } from "./ui/Caption";

const browse = ["milestone", "issue", "decision", "learning", "thread", "inbox", "convention"];

// Linear's sidebar: quiet caps captions, soft-fill selection, no accent bar.
export function Shell() {
  const [isPaletteOpen, setPaletteOpen] = useState(false);
  const { data } = useNav();

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setPaletteOpen((o) => !o); }
      if (e.key === "/" && !(e.target instanceof HTMLInputElement) && !(e.target instanceof HTMLTextAreaElement)) { e.preventDefault(); setPaletteOpen(true); }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, []);

  return (
    <div className="grid min-h-dvh grid-cols-[232px_minmax(0,1fr)]">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:left-2 focus:top-2 focus:z-50 focus:rounded-md focus:bg-raised focus:px-3 focus:py-2">Skip to content</a>
      <aside className="sticky top-0 flex h-dvh flex-col gap-5 border-r border-line bg-panel px-3 py-3">
        <div className="flex h-8 items-center gap-2 px-2">
          <span className="grid size-5 place-items-center rounded-[5px] bg-fg text-[11px] font-bold text-bg" aria-hidden>a</span>
          <span className="text-sm font-semibold text-fg">anvil</span>
        </div>
        <button onClick={() => setPaletteOpen(true)} className="flex h-8 items-center gap-2 rounded-md border border-line bg-bg px-2 text-sm text-faint hover:border-line-strong hover:text-dim">
          <Search size={14} aria-hidden /> Go to…
          <kbd className="ml-auto rounded-sm border border-line-strong px-1 text-cap">⌘K</kbd>
        </button>
        <nav className="flex flex-col gap-px" aria-label="Main">
          <NavLink to="/" isExact icon={<Home size={15} className="text-dim" aria-hidden />}>Now</NavLink>
          <NavLink to="/knowledge" icon={<BookOpen size={15} className="text-dim" aria-hidden />}>Knowledge</NavLink>
        </nav>
        <nav className="flex flex-col gap-px" aria-label="Projects">
          <Caption className="mb-1 px-2">Projects</Caption>
          {data?.projects.map((p) => (
            <Link key={p.slug} to="/p/$slug" params={{ slug: p.slug }} className={navItem}>
              <span className="grid size-4 place-items-center rounded-sm bg-raised text-[10px] font-semibold uppercase text-dim" aria-hidden>{p.slug[0]}</span>
              <span className="truncate">{p.slug}</span>
              {p.inProgress > 0 && (
                <span className="ml-auto flex items-center gap-1 text-xs text-dim num" title={`${p.inProgress} in progress`}>
                  <span className="size-1.5 rounded-full bg-s-progress" aria-hidden />{p.inProgress}
                </span>
              )}
            </Link>
          ))}
        </nav>
        <nav className="flex flex-col gap-px" aria-label="Browse">
          <Caption className="mb-1 px-2">Browse</Caption>
          {browse.map((t) => (
            <Link key={t} to="/t/$type" params={{ type: t }} className={navItem}>
              <span className="capitalize">{t === "inbox" ? "Inbox" : `${t}s`}</span>
            </Link>
          ))}
        </nav>
        <div className="mt-auto px-2 text-cap text-faint">read-only · live from the vault</div>
      </aside>
      <main id="main" className="min-w-0">
        <Outlet />
      </main>
      <Palette isOpen={isPaletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  );
}
