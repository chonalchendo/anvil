import { cn } from "../../lib/cn";

// Status is shape plus hue at a fixed column (Linear). Shape alone separates
// the states for readers who cannot see the hue.
type Shape = "hollow" | "dashed" | "half" | "check" | "cross" | "slash" | "warn" | "dot" | "pause";
const shapeOf: Record<string, Shape> = {
  open: "hollow", active: "dot", planned: "dashed", proposed: "dashed", draft: "dashed", raw: "dashed",
  "in-progress": "half", paused: "pause",
  resolved: "check", done: "check", accepted: "check", verified: "check", closed: "check", promoted: "check",
  escalated: "warn",
  abandoned: "cross", dropped: "cross", rejected: "cross", retired: "cross", retracted: "cross",
  superseded: "slash", stale: "slash",
};
const filled = new Set<Shape>(["check", "cross", "slash"]);

export interface GlyphProps {
  status: string | null;
  size?: number;
  className?: string;
}

export function Glyph({ status, size = 14, className }: GlyphProps) {
  const shape = shapeOf[status ?? ""] ?? "hollow";
  const c = "var(--s)";
  return (
    <svg width={size} height={size} viewBox="0 0 14 14" aria-label={status ?? "no status"} role="img" className={cn(`s-${status ?? "none"}`, "shrink-0", className)}>
      {shape === "warn" ? (
        <path d="M7 1.5 13 12.5H1Z" fill="none" stroke={c} strokeWidth="1.5" strokeLinejoin="round" />
      ) : (
        <circle cx="7" cy="7" r="5.75" fill={filled.has(shape) ? c : "none"} stroke={c} strokeWidth="1.5" strokeDasharray={shape === "dashed" ? "2.2 2" : undefined} />
      )}
      {shape === "half" && <path d="M7 3.5a3.5 3.5 0 0 1 0 7Z" fill={c} />}
      {shape === "dot" && <circle cx="7" cy="7" r="2.5" fill={c} />}
      {shape === "pause" && <path d="M5.5 4.5v5M8.5 4.5v5" stroke={c} strokeWidth="1.5" />}
      {shape === "check" && <path d="m4.4 7.2 1.8 1.8 3.5-3.6" fill="none" stroke="var(--color-bg)" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />}
      {shape === "cross" && <path d="m5 5 4 4m0-4-4 4" stroke="var(--color-bg)" strokeWidth="1.5" strokeLinecap="round" />}
      {shape === "slash" && <path d="m4.8 9.2 4.4-4.4" stroke="var(--color-bg)" strokeWidth="1.5" strokeLinecap="round" />}
      {shape === "warn" && <path d="M7 5.5v3M7 10.2v.1" stroke={c} strokeWidth="1.5" strokeLinecap="round" />}
    </svg>
  );
}
