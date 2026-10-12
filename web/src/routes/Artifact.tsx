import { useParams } from "@tanstack/react-router";
import { useArtifact } from "../lib/api";
import { ArtifactView } from "../components/artifact/ArtifactView";
import { QueryStatus } from "../components/ui/QueryStatus";

export function ArtifactPage() {
  const { key } = useParams({ from: "/a/$key" });
  const { data, error } = useArtifact(key);
  return data ? <ArtifactView artifact={data} /> : <QueryStatus error={error} what="artifact" />;
}
