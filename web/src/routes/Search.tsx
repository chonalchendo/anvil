import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { Search as SearchIcon } from "lucide-react";
import { Fragment, useEffect, useState } from "react";
import { useSearch as useHits, typeWord } from "../lib/api";
import { Caption, Glyph, PageHead, TypeIcon } from "../components/ui";

// The server marks hits with \u0001…\u0002 so the snippet stays plain text
// until here; nothing from a body is ever parsed as HTML.
function Snippet({ text }: { text: string }) {
  const parts = text.split(/(\u0001[^\u0002]*\u0002)/);
  return (
    <>
      {parts.map((p, i) =>
        p.startsWith("\u0001") ? <mark key={i}>{p.slice(1, -1)}</mark> : <Fragment key={i}>{p}</Fragment>,
      )}
    </>
  );
}

export function SearchPage() {
  const { q = "" } = useSearch({ strict: false }) as { q?: string };
  const [term, setTerm] = useState(q);
  const navigate = useNavigate();
  const { data, isFetching } = useHits(q);
  useEffect(() => setTerm(q), [q]);

  const groups = new Map<string, NonNullable<typeof data>["hits"]>();
  for (const h of data?.hits ?? []) (groups.get(h.type) ?? groups.set(h.type, []).get(h.type)!).push(h);

  return (
    <div className="mx-auto max-w-[920px] px-10 py-10">
      <PageHead title="Search" lede="Every body, title and goal in the vault, ranked." />
      <form
        role="search"
        onSubmit={(e) => { e.preventDefault(); navigate({ to: "/search", search: { q: term.trim() } as any }); }}
        className="mb-8 flex h-10 items-center gap-2 rounded-lg border border-line-strong bg-panel px-3 focus-within:border-accent"
      >
        <SearchIcon size={15} className="text-faint" aria-hidden />
        <input
          value={term} onChange={(e) => setTerm(e.target.value)} autoFocus aria-label="Search terms" name="q" type="search" autoComplete="off" spellCheck={false}
          placeholder="verification predicate…" className="flex-1 bg-transparent text-base text-fg outline-none placeholder:text-faint"
        />
      </form>
      <div aria-live="polite">
        {q && data && (
          <p className="mb-6 text-sm text-dim num">
            {data.hits.length ? `${data.hits.length}${data.hits.length >= 60 ? "+" : ""} matches for “${q}”` : `No matches for “${q}”.`}
          </p>
        )}
        {isFetching && !data && <p className="text-dim">Searching…</p>}
      </div>
      {[...groups.entries()].map(([type, hits]) => (
        <section key={type} className="mb-8">
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
                  <p className="mt-1 pl-5 text-sm text-dim line-clamp-2"><Snippet text={h.snippet} /></p>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
