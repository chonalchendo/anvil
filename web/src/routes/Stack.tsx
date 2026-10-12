import { useParams } from "@tanstack/react-router";
import { useArtifact, useStack } from "../lib/api";
import { DocHeader } from "../components/artifact/DocHeader";
import { StackLayers } from "../components/StackLayers";
import { QueryStatus } from "../components/ui/QueryStatus";

export function StackPage() {
  const { key } = useParams({ from: "/a/$key/stack" });
  const artifact = useArtifact(key);
  const stack = useStack(key);
  if (!artifact.data) return <QueryStatus error={artifact.error} what="issue" />;
  return (
    <div className="mx-auto max-w-[960px] px-10 py-10">
      <DocHeader artifact={artifact.data} current="stack" />
      {stack.data ? <StackLayers stack={stack.data} /> : <QueryStatus error={stack.error} what="context stack" />}
    </div>
  );
}
