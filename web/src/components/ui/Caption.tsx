import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Caption({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("text-cap font-medium uppercase tracking-[0.04em] text-faint", className)}>{children}</div>;
}
