import { useParams } from "@tanstack/react-router";
import { DiagramFrame } from "../components/DiagramFrame";

export function DiagramPage() {
  const { name } = useParams({ from: "/diagram/$name" });
  return (
    <div className="flex min-h-dvh flex-col">
      <div className="flex h-11 items-center border-b border-line px-6 text-sm text-dim">
        <button onClick={() => history.back()} className="hover:text-fg">← Back</button>
        <span className="ml-4 font-mono text-xs text-faint" translate="no">{name}</span>
      </div>
      <div className="p-6"><DiagramFrame name={name} /></div>
    </div>
  );
}
