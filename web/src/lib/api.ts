import { useQuery } from "@tanstack/react-query";
import { hc, type InferResponseType } from "hono/client";
import type { Api } from "../../server/api";

// The one typed client: every response shape below derives from the server's
// routes, so a server change that breaks the client fails the type-check.
const client = hc<Api>("/api");

export type Nav = InferResponseType<typeof client.nav.$get, 200>;
export type Home = InferResponseType<typeof client.home.$get, 200>;
export type Project = InferResponseType<(typeof client.project)[":slug"]["$get"], 200>;
export type Artifact = InferResponseType<(typeof client.artifact)[":key"]["$get"], 200>;
export type Stack = InferResponseType<(typeof client.stack)[":key"]["$get"], 200>;
export type List = InferResponseType<(typeof client.list)[":type"]["$get"], 200>;
export type Search = InferResponseType<typeof client.search.$get, 200>;
export type Knowledge = InferResponseType<typeof client.knowledge.$get, 200>;
export type Entry = InferResponseType<typeof client.palette.$get, 200>[number];

export type Item = Artifact["graph"]["nodes"][number];
export type IssueRow = Project["loose"][number];
export type Milestone = Project["milestones"][number];
export type Described = Home["recent"][number];
export type ProjectSummary = Nav["projects"][number];
export type Linked = Artifact["backlinks"][number];
export type Hit = Search["hits"][number];
export type Judged = Artifact["judge"][number];

export class NotFound extends Error {
  name = "NotFound";
}

async function body<T>(res: Promise<Response>): Promise<T> {
  const r = await res;
  if (r.status === 404) throw new NotFound(r.url);
  if (!r.ok) throw new Error(`${r.url}: ${r.status}`);
  return r.json() as Promise<T>;
}

const live = { refetchInterval: 15000 };

export const useNav = () => useQuery({ queryKey: ["nav"], queryFn: () => body<Nav>(client.nav.$get()), ...live });
export const useHome = () => useQuery({ queryKey: ["home"], queryFn: () => body<Home>(client.home.$get()), ...live });
export const useProject = (slug: string) =>
  useQuery({ queryKey: ["project", slug], queryFn: () => body<Project>(client.project[":slug"].$get({ param: { slug } })), ...live });
export const useArtifact = (key: string) =>
  useQuery({ queryKey: ["artifact", key], queryFn: () => body<Artifact>(client.artifact[":key"].$get({ param: { key } })), ...live });
export const useStack = (key: string) =>
  useQuery({ queryKey: ["stack", key], queryFn: () => body<Stack>(client.stack[":key"].$get({ param: { key } })) });
export interface ListQuery { project?: string; status?: string; topic?: string }
export const useList = (type: string, { project, status, topic }: ListQuery) =>
  useQuery({
    queryKey: ["list", type, project, status, topic],
    queryFn: () => body<List>(client.list[":type"].$get({ param: { type }, query: { project, status, topic } })),
    ...live,
  });
export const useSearch = (q: string) =>
  useQuery({ queryKey: ["search", q], queryFn: () => body<Search>(client.search.$get({ query: { q } })), enabled: !!q });
export const useKnowledge = (type: string) =>
  useQuery({ queryKey: ["knowledge", type], queryFn: () => body<Knowledge>(client.knowledge.$get({ query: type ? { type } : {} })), ...live });
export const usePalette = (enabled: boolean) =>
  useQuery({ queryKey: ["palette"], queryFn: () => body<Entry[]>(client.palette.$get()), enabled, staleTime: 30000 });

export function compact(n: number) {
  if (n >= 1e6) return `${(n / 1e6).toFixed(n >= 1e7 ? 0 : 1)}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(n >= 1e4 ? 0 : 1)}k`;
  return String(n);
}

// Relative age reads faster than a date in a dense row; the exact date is the title.
export function age(d: string | null) {
  if (!d) return "";
  const days = Math.round((Date.now() - new Date(d).getTime()) / 864e5);
  if (days <= 0) return "today";
  if (days < 30) return `${days}d`;
  if (days < 365) return `${Math.round(days / 30)}mo`;
  return `${Math.round(days / 365)}y`;
}

export const typeWord = (t: string) => t.replace("-", " ");
