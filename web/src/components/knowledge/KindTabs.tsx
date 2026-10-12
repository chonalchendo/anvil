import { Link } from "@tanstack/react-router";

const kinds = [
  { v: "", label: "Everything" },
  { v: "decision", label: "Decisions" },
  { v: "learning", label: "Learnings" },
  { v: "thread", label: "Threads" },
];

export function KindTabs({ current }: { current: string }) {
  return (
    <nav className="mb-4 flex gap-1" aria-label="Kind">
      {kinds.map((k) => (
        <Link
          key={k.v} to="/knowledge" search={k.v ? { type: k.v } : {}} data-on={current === k.v} aria-current={current === k.v ? "page" : undefined}
          className="inline-block h-7 rounded-md px-2.5 text-sm leading-7 text-dim hover:text-fg data-[on=true]:bg-raised data-[on=true]:text-fg"
        >
          {k.label}
        </Link>
      ))}
    </nav>
  );
}
