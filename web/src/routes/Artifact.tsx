import * as Collapsible from "@radix-ui/react-collapsible";
import { Link, useParams } from "@tanstack/react-router";
import { ChevronRight, Layers } from "lucide-react";
import { useEffect, useState } from "react";
import { useArtifact, typeWord, type Artifact, type Linked } from "../lib/api";
import { LocalGraph } from "../components/LocalGraph";
import { DiagramFrame } from "../components/DiagramFrame";
import { Caption, Glyph, IssueRows, Loading, Pill, TypeIcon } from "../components/ui";

export function ArtifactPage() {
  const { key } = useParams({ from: "/a/$key" });
  const { data, error } = useArtifact(key);
  if (error) return <div className="p-10 text-dim">No artifact {key}.</div>;
  if (!data) return <Loading />;
  return <Page a={data} />;
}

export function DocHeader({ a, tabs }: { a: Pick<Artifact, "key" | "type" | "title" | "status" | "project" | "updated" | "crumbs" | "description" | "tags">; tabs?: "doc" | "stack" }) {
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
        {a.tags?.map((t) => <Pill key={t}>{t}</Pill>)}
      </div>
      {tabs && (
        <div className="mt-6 flex gap-1 border-b border-line" role="tablist">
          <Link to="/a/$key" params={{ key: a.key }} role="tab" aria-selected={tabs === "doc"} className="-mb-px border-b-2 border-transparent px-3 pb-2 text-sm text-dim hover:text-fg aria-selected:border-fg aria-selected:text-fg">
            Document
          </Link>
          <Link to="/a/$key/stack" params={{ key: a.key }} role="tab" aria-selected={tabs === "stack"} className="-mb-px flex items-center gap-1.5 border-b-2 border-transparent px-3 pb-2 text-sm text-dim hover:text-fg aria-selected:border-fg aria-selected:text-fg">
            <Layers size={13} aria-hidden /> Context stack
          </Link>
        </div>
      )}
    </header>
  );
}

function Page({ a }: { a: Artifact }) {
  return (
    <div className="mx-auto grid max-w-[1240px] grid-cols-[minmax(0,1fr)_280px] gap-14 px-10 py-10 max-lg:grid-cols-1">
      <article className="min-w-0">
        <DocHeader a={a} tabs={a.type === "issue" ? "doc" : undefined} />
        {a.diagrams.length > 0 && (
          <div className="mb-8 flex flex-col gap-3">
            {a.diagrams.map((n) => (
              <figure key={n}>
                <div className="overflow-hidden rounded-lg border border-line bg-panel">
                  <DiagramFrame name={n} />
                </div>
                <figcaption className="mt-1.5 flex justify-between text-xs text-dim">
                  <span translate="no">{n}</span>
                  <Link to="/diagram/$name" params={{ name: n }} className="hover:text-fg">Open full size →</Link>
                </figcaption>
              </figure>
            ))}
          </div>
        )}
        {a.issues && (
          <section className="mb-10">
            <Caption className="mb-2">Issues · {a.issues.length}</Caption>
            <div className="rounded-lg border border-line bg-panel py-1"><IssueRows rows={a.issues} /></div>
          </section>
        )}
        <div className="prose" dangerouslySetInnerHTML={{ __html: a.html }} />
      </article>
      <Rail a={a} />
    </div>
  );
}

// Scroll-spy: the heading nearest above the top third of the viewport is current.
function useCurrent(ids: string[]) {
  const [current, setCurrent] = useState<string | null>(null);
  useEffect(() => {
    const onScroll = () => {
      let cur: string | null = null;
      for (const id of ids) {
        const el = document.getElementById(id);
        if (el && el.getBoundingClientRect().top < window.innerHeight / 3) cur = id;
      }
      setCurrent(cur);
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [ids.join()]);
  return current;
}

function Rail({ a }: { a: Artifact }) {
  const current = useCurrent(a.toc.map((t) => t.id));
  return (
    <aside className="sticky top-0 flex max-h-dvh flex-col gap-7 overflow-y-auto py-10 max-lg:static max-lg:max-h-none" aria-label="About this artifact">
      {a.judge.length > 0 && (
        <section>
          <Caption className="mb-2">Judge</Caption>
          <dl className="grid grid-cols-[96px_minmax(0,1fr)] gap-y-1.5 text-sm">
            {a.judge.map((j, i) => (
              <div key={i} className="contents">
                <dt className="text-dim">{j.label}</dt>
                <dd className={`break-words num ${j.mono ? "font-mono text-xs leading-[19.5px]" : ""} ${j.tone === "dim" ? "text-faint" : "text-fg"}`} title={j.title || j.value}>
                  {j.tone && j.tone !== "dim" ? <Pill tone={j.tone}>{j.value}</Pill>
                    : j.href ? <a href={j.href} target={j.href.startsWith("http") ? "_blank" : undefined} rel="noreferrer" className="underline decoration-line-strong underline-offset-2 hover:decoration-fg">{j.value}</a>
                    : j.value}
                </dd>
              </div>
            ))}
          </dl>
        </section>
      )}

      {a.toc.length > 2 && (
        <nav aria-label="On this page">
          <Caption className="mb-2">On this page</Caption>
          <ul className="flex flex-col border-l border-line">
            {a.toc.map((t) => (
              <li key={t.id}>
                <a
                  href={`#${t.id}`}
                  aria-current={current === t.id ? "location" : undefined}
                  className={`-ml-px block border-l py-1 text-sm leading-[18px] hover:text-fg ${t.depth === 3 ? "pl-6 text-faint" : "pl-3 text-dim"} ${current === t.id ? "border-fg text-fg" : "border-transparent"}`}
                >
                  {t.text}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      )}

      <LinkList title="Links to" rows={a.forward} />
      <LinkList title="Cited by" rows={a.backlinks} />

      <section>
        <Caption className="mb-2">Neighbourhood</Caption>
        <LocalGraph graph={a.graph} center={a.key} />
      </section>

      <Collapsible.Root>
        <Collapsible.Trigger className="group flex items-center gap-1 text-cap font-medium uppercase tracking-[0.04em] text-faint hover:text-dim">
          <ChevronRight size={12} className="group-data-[state=open]:rotate-90" /> All properties
        </Collapsible.Trigger>
        <Collapsible.Content>
          <dl className="mt-2 grid grid-cols-[110px_minmax(0,1fr)] gap-x-2 gap-y-1 text-xs">
            {a.props.map((p) => (
              <div key={p.k} className="contents">
                <dt className="truncate font-mono text-faint" translate="no">{p.k}</dt>
                <dd className="break-words text-dim">{Array.isArray(p.v) ? p.v.join(", ") : p.v}</dd>
              </div>
            ))}
          </dl>
        </Collapsible.Content>
      </Collapsible.Root>
    </aside>
  );
}

function LinkList({ title, rows }: { title: string; rows: Linked[] }) {
  const [all, setAll] = useState(false);
  if (!rows.length) return null;
  const shown = all ? rows : rows.slice(0, 8);
  return (
    <section>
      <Caption className="mb-2">{title} · <span className="num">{rows.length}</span></Caption>
      <ul className="flex flex-col">
        {shown.map((r) => (
          <li key={r.key}>
            <Link to="/a/$key" params={{ key: r.key }} className="flex items-start gap-2 rounded-md px-1 py-1 text-sm leading-[18px] text-body hover:bg-hover hover:text-fg" title={`${typeWord(r.type)} · ${r.relation}`}>
              <Glyph status={r.status} size={12} className="mt-[3px]" />
              <span className="line-clamp-2">{r.title}</span>
            </Link>
          </li>
        ))}
      </ul>
      {rows.length > 8 && (
        <button onClick={() => setAll((s) => !s)} className="mt-1 px-1 text-xs text-faint hover:text-dim">
          {all ? "Show fewer" : `Show all ${rows.length}`}
        </button>
      )}
    </section>
  );
}
