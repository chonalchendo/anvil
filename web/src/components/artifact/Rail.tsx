import type { Artifact } from "../../lib/api";
import { LocalGraph } from "../LocalGraph";
import { Caption } from "../ui/Caption";
import { JudgeList } from "./JudgeList";
import { LinkList } from "./LinkList";
import { PropertyList } from "./PropertyList";
import { TocNav } from "./TocNav";

// Linear's property column: metadata sits beside the reading column, so
// neither interrupts the other.
export function Rail({ artifact: a }: { artifact: Artifact }) {
  return (
    <aside className="sticky top-0 flex max-h-dvh flex-col gap-7 overflow-y-auto overscroll-contain py-10 max-lg:static max-lg:max-h-none" aria-label="About this artifact">
      <JudgeList judge={a.judge} />
      <TocNav toc={a.toc} />
      <LinkList title="Links to" rows={a.forward} />
      <LinkList title="Cited by" rows={a.backlinks} />
      <section>
        <Caption className="mb-2">Neighbourhood</Caption>
        <LocalGraph graph={a.graph} center={a.key} />
      </section>
      <PropertyList props={a.props} />
    </aside>
  );
}
