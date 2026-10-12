import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

export const navItem =
  "flex h-7 items-center gap-2.5 rounded-md px-2 text-sm text-body hover:bg-hover hover:text-fg [&.active]:bg-raised [&.active]:text-fg";

export interface NavLinkProps {
  to: string;
  icon?: ReactNode;
  isExact?: boolean;
  children: ReactNode;
}

export function NavLink({ to, icon, isExact, children }: NavLinkProps) {
  return (
    <Link to={to} className={navItem} activeOptions={{ exact: isExact }}>
      {icon}
      {children}
    </Link>
  );
}
