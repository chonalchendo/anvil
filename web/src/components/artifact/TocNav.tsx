import type { Artifact } from "../../lib/api";
import { cn } from "../../lib/cn";
import { useScrollSpy } from "../../hooks/useScrollSpy";
import { Caption } from "../ui/Caption";

export function TocNav({ toc }: { toc: Artifact["toc"] }) {
  const current = useScrollSpy(toc.map((t) => t.id));
  if (toc.length <= 2) return null;
  return (
    <nav aria-label="On this page">
      <Caption className="mb-2">On this page</Caption>
      <ul className="flex flex-col border-l border-line">
        {toc.map((t) => (
          <li key={t.id}>
            <a
              href={`#${t.id}`}
              aria-current={current === t.id ? "location" : undefined}
              className={cn(
                "-ml-px block border-l border-transparent py-1 text-sm leading-[18px] hover:text-fg",
                t.depth === 3 ? "pl-6 text-faint" : "pl-3 text-dim",
                current === t.id && "border-fg text-fg",
              )}
            >
              {t.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
