import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface PillProps {
  /** A status class (`s-<tone>`) gives the pill its hue; without one it is neutral. */
  tone?: string;
  title?: string;
  children: ReactNode;
}

// Tinted pill: same-hue fill at 12% and border at 30% (GitHub Projects).
export function Pill({ tone, title, children }: PillProps) {
  return (
    <span
      title={title}
      className={cn(
        "inline-flex h-[18px] items-center gap-1 whitespace-nowrap rounded-sm border px-1.5 text-cap font-medium num",
        tone ? [`s-${tone}`, "border-[color-mix(in_srgb,var(--s)_30%,transparent)] bg-[color-mix(in_srgb,var(--s)_12%,transparent)] text-[var(--s)]"] : "border-line-strong text-dim",
      )}
    >
      {children}
    </span>
  );
}
