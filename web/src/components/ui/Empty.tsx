import type { ReactNode } from "react";

export function Empty({ children }: { children: ReactNode }) {
  return <div className="rounded-lg border border-dashed border-line-strong px-4 py-6 text-center text-dim">{children}</div>;
}
