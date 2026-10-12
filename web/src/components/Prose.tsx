import { useNavigate } from "@tanstack/react-router";
import type { MouseEvent } from "react";

// Rendered markdown. The server sanitises it (no raw HTML, no unsafe URLs);
// a same-origin `/a/` link routes in the client instead of reloading the page.
export function Prose({ html }: { html: string }) {
  const navigate = useNavigate();
  const handleClick = (e: MouseEvent<HTMLDivElement>) => {
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (!href?.startsWith("/a/") || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    navigate({ to: "/a/$key", params: { key: decodeURIComponent(href.slice(3)) } });
  };
  return <div className="prose" onClick={handleClick} dangerouslySetInnerHTML={{ __html: html }} />;
}
