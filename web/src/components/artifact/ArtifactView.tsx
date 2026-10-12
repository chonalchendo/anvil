import { Link } from "@tanstack/react-router";
import type { Artifact } from "../../lib/api";
import { DiagramFrame } from "../DiagramFrame";
import { Prose } from "../Prose";
import { Caption } from "../ui/Caption";
import { IssueRows } from "../ui/IssueRows";
import { DocHeader } from "./DocHeader";
import { Rail } from "./Rail";

export function ArtifactView({ artifact: a }: { artifact: Artifact }) {
  return (
    <div className="mx-auto grid max-w-[1240px] grid-cols-[minmax(0,1fr)_280px] gap-14 px-10 py-10 max-lg:grid-cols-1">
      <article className="min-w-0">
        <DocHeader artifact={a} current={a.type === "issue" ? "doc" : undefined} />
        {a.diagrams.length > 0 && (
          <div className="mb-8 flex flex-col gap-3">
            {a.diagrams.map((n) => (
              <figure key={n}>
                <div className="overflow-hidden rounded-lg border border-line bg-panel"><DiagramFrame name={n} /></div>
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
        <Prose html={a.html} />
      </article>
      <Rail artifact={a} />
    </div>
  );
}
