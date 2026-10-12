import type { ReactNode } from "react";

export interface PageHeadProps {
  crumbs?: ReactNode;
  title: ReactNode;
  lede?: ReactNode;
  children?: ReactNode;
}

// Stripe's head stack: orientation, then identity, then the promise.
export function PageHead({ crumbs, title, lede, children }: PageHeadProps) {
  return (
    <header className="mb-8">
      {crumbs && <nav aria-label="Breadcrumb" className="mb-3 flex items-center gap-1.5 text-xs text-dim">{crumbs}</nav>}
      <h1 className="text-xl font-semibold tracking-[-0.012em]">{title}</h1>
      {lede && <p className="mt-2 max-w-[68ch] text-base text-dim">{lede}</p>}
      {children}
    </header>
  );
}
