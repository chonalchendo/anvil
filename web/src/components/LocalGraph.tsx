import { forceCenter, forceCollide, forceLink, forceManyBody, forceSimulation, type SimulationNodeDatum } from "d3-force";
import { useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { typeWord, type Artifact } from "../lib/api";
import { cn } from "../lib/cn";

type N = SimulationNodeDatum & { key: string; title: string; type: string; status: string | null };
const W = 280;
const H = 220;

// Quartz's local graph, with the decoration removed: node hue is the target's
// status and the centre is the page, so the graph answers "what is around this
// and is it alive" rather than drawing a constellation.
export function LocalGraph({ graph, center }: { graph: Artifact["graph"]; center: string }) {
  const navigate = useNavigate();
  const [hover, setHover] = useState<N | null>(null);
  const { nodes, links } = useMemo(() => {
    const nodes: N[] = graph.nodes.map((n) => ({ ...n, ...(n.key === center ? { fx: W / 2, fy: H / 2 } : {}) }));
    const links = graph.edges.map((e) => ({ ...e }));
    forceSimulation(nodes)
      .force("link", forceLink<N, { source: string | N; target: string | N }>(links).id((d) => d.key).distance(52))
      .force("charge", forceManyBody().strength(-90))
      .force("collide", forceCollide(9))
      .force("center", forceCenter(W / 2, H / 2))
      .stop()
      .tick(240);
    for (const n of nodes) {
      n.x = Math.max(8, Math.min(W - 8, n.x!));
      n.y = Math.max(8, Math.min(H - 8, n.y!));
    }
    // forceLink swaps each edge's ids for the node objects in place.
    return { nodes, links: links as unknown as { source: N; target: N; relation: string }[] };
  }, [graph, center]);

  if (nodes.length < 2) return <div className="text-xs text-faint">No linked artifacts.</div>;
  return (
    <div className="relative">
      <svg width={W} height={H} className="rounded-md border border-line bg-panel" role="img" aria-label={`${nodes.length - 1} linked artifacts`}>
        {links.map((l, i) => (
          <line
            key={i} x1={l.source.x} y1={l.source.y} x2={l.target.x} y2={l.target.y}
            stroke={hover && (l.source === hover || l.target === hover) ? "rgb(255 255 255 / .45)" : "rgb(255 255 255 / .1)"}
            strokeDasharray={l.relation === "body" ? "2 3" : undefined}
          />
        ))}
        {nodes.map((n) => (
          <circle
            key={n.key} cx={n.x} cy={n.y} r={n.key === center ? 7 : 4.5}
            className={cn(`s-${n.status ?? "none"}`, "cursor-pointer")}
            fill="var(--s)" stroke={n.key === center ? "var(--color-fg)" : "none"} strokeWidth={1.5}
            onMouseEnter={() => setHover(n)} onMouseLeave={() => setHover(null)}
            onClick={() => n.key !== center && navigate({ to: "/a/$key", params: { key: n.key } })}
          >
            <title>{`${typeWord(n.type)}: ${n.title}`}</title>
          </circle>
        ))}
      </svg>
      <div className="mt-1.5 h-4 truncate text-xs text-dim" aria-live="polite">
        {hover ? `${typeWord(hover.type)} · ${hover.title}` : "Solid edge: frontmatter link. Dotted: body link."}
      </div>
    </div>
  );
}
