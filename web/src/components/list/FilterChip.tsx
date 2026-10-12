import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import type { ListQuery } from "../../lib/api";

export interface FilterChipProps {
  type: string;
  search: ListQuery;
  isOn: boolean;
  children: ReactNode;
}

export function FilterChip({ type, search, isOn, children }: FilterChipProps) {
  return (
    <Link
      to="/t/$type" params={{ type }} search={search} data-on={isOn} aria-current={isOn ? "page" : undefined}
      className="flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm text-dim hover:text-fg data-[on=true]:bg-raised data-[on=true]:text-fg"
    >
      {children}
    </Link>
  );
}
