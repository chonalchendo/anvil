import { compact, type Judged } from "../../lib/api";
import { cn } from "../../lib/cn";
import { Caption } from "../ui/Caption";
import { Pill } from "../ui/Pill";

function Value({ j }: { j: Judged }) {
  const text = j.count != null ? compact(j.count) : j.value;
  if (j.tone === "pass" || j.tone === "fail") return <Pill tone={j.tone}>{text}</Pill>;
  if (j.href)
    return (
      <a href={j.href} target={j.href.startsWith("http") ? "_blank" : undefined} rel="noreferrer" className="underline decoration-line-strong underline-offset-2 hover:decoration-fg">
        {text}
      </a>
    );
  return <>{text}</>;
}

// The fields a reader judges this artifact by, first in the rail (Linear's
// property column).
export function JudgeList({ judge }: { judge: Judged[] }) {
  if (!judge.length) return null;
  return (
    <section>
      <Caption className="mb-2">Judge</Caption>
      <dl className="grid grid-cols-[96px_minmax(0,1fr)] gap-y-1.5 text-sm">
        {judge.map((j, i) => (
          <div key={i} className="contents">
            <dt className="text-dim">{j.label}</dt>
            <dd className={cn("break-words num", j.mono && "font-mono text-xs leading-[19.5px]", j.tone === "dim" ? "text-faint" : "text-fg")} title={j.title}>
              <Value j={j} />
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
