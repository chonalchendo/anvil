import { useNavigate, useSearch } from "@tanstack/react-router";
import { Search as SearchIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { useSearch as useHits, type Hit } from "../lib/api";
import { HitGroup } from "../components/search/HitGroup";
import { PageHead } from "../components/ui/PageHead";

export function SearchPage() {
  const { q = "" } = useSearch({ from: "/search" });
  const [term, setTerm] = useState(q);
  const navigate = useNavigate();
  const { data, error, isFetching } = useHits(q);
  useEffect(() => setTerm(q), [q]);

  const groups = new Map<string, Hit[]>();
  for (const h of data?.hits ?? []) groups.set(h.type, [...(groups.get(h.type) ?? []), h]);

  return (
    <div className="mx-auto max-w-[920px] px-10 py-10">
      <PageHead title="Search" lede="Every body, title and goal in the vault, ranked." />
      <form
        role="search"
        onSubmit={(e) => { e.preventDefault(); navigate({ to: "/search", search: { q: term.trim() || undefined } }); }}
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
        {error && <p className="text-dim" role="alert">Search failed: {error.message}. Check that the server is running.</p>}
      </div>
      {[...groups.entries()].map(([type, hits]) => <HitGroup key={type} type={type} hits={hits} />)}
    </div>
  );
}
