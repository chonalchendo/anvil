import { useEffect, useRef, useState } from "react";
import { cn } from "../lib/cn";

// Diagrams are fixed-width HTML. The frame is sandboxed without scripts but
// same-origin, so the parent can read the diagram's natural size and scale it
// to fit the column instead of cropping it.
export function DiagramFrame({ name, maxHeight, interactive = true }: { name: string; maxHeight?: number; interactive?: boolean }) {
  const box = useRef<HTMLDivElement>(null);
  const frame = useRef<HTMLIFrameElement>(null);
  const [size, setSize] = useState<{ w: number; h: number } | null>(null);
  const [width, setWidth] = useState(0);

  useEffect(() => {
    const ro = new ResizeObserver(([e]) => setWidth(e.contentRect.width));
    if (box.current) ro.observe(box.current);
    return () => ro.disconnect();
  }, []);

  const measure = () => {
    const d = frame.current?.contentDocument?.documentElement;
    if (d) setSize({ w: Math.max(d.scrollWidth, 320), h: Math.max(d.scrollHeight, 200) });
  };

  let scale = size && width ? Math.min(1, width / size.w) : 1;
  if (size && maxHeight) scale = Math.min(scale, maxHeight / size.h);
  return (
    <div ref={box} className="relative w-full overflow-hidden" style={{ height: size ? size.h * scale : maxHeight ?? 360 }}>
      <iframe
        ref={frame} src={`/diagram-src/${name}`} sandbox="allow-same-origin" title={`Diagram ${name}`}
        loading="lazy" onLoad={measure} tabIndex={interactive ? 0 : -1}
        className={cn("absolute left-0 top-0 origin-top-left border-0", !interactive && "pointer-events-none")}
        style={size ? { width: size.w, height: size.h, transform: `scale(${scale})` } : { width: "100%", height: "100%" }}
      />
    </div>
  );
}
