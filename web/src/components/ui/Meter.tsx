import { cn } from "../../lib/cn";

export interface MeterProps {
  done: number;
  total: number;
  isLive?: boolean;
}

// A thin progress meter replaces Grafana's full-saturation tile: area of hue
// for state, the number beside it for the value.
export function Meter({ done, total, isLive }: MeterProps) {
  const pct = total ? Math.round((done / total) * 100) : 0;
  return (
    <span className="inline-flex items-center gap-2" title={`${done} of ${total} resolved`}>
      <span className="relative h-1 w-16 overflow-hidden rounded-full bg-line-strong" aria-hidden>
        <span className={cn("absolute inset-y-0 left-0 rounded-full", isLive ? "bg-s-progress" : "bg-s-done")} style={{ width: `${pct}%` }} />
      </span>
      <span className="num text-xs text-dim">{done}/{total}</span>
    </span>
  );
}
