import { useEffect, useState } from "react";

// The heading nearest above the top third of the viewport is current.
export function useScrollSpy(ids: string[]): string | null {
  const [current, setCurrent] = useState<string | null>(null);
  const key = ids.join("\n");
  useEffect(() => {
    const list = key ? key.split("\n") : [];
    const handleScroll = () => {
      let cur: string | null = null;
      for (const id of list) {
        const el = document.getElementById(id);
        if (el && el.getBoundingClientRect().top < window.innerHeight / 3) cur = id;
      }
      setCurrent(cur);
    };
    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, [key]);
  return current;
}
