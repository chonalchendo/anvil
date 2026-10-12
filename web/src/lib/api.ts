import { useQuery } from "@tanstack/react-query";

export type Item = {
  key: string; type: string; status: string | null; project: string | null; title: string;
  created: string | null; updated: string | null;
};
export type IssueRow = Item & {
  ordinal: string; owner: string | null; severity: string | null; verdict: string | null;
  prs: { n: number; url: string }[]; rounds: number | null; tokens: number | null;
  diff: number | null; files: number | null; goal: string | null;
};
export type Milestone = Item & {
  goal: string | null; approved: string | null; done: string | null; measured: string | null;
  resolved: number; total: number; counts: Record<string, number>; live: boolean; issues: IssueRow[];
};
export type Described = Item & { description: string | null; confidence?: string | null; topic?: string | null };
export type ProjectSummary = {
  slug: string; description: string | null; inProgress: number; open: number;
  milestonesInFlight: number; milestonesPlanned: number; updated: string;
};
export type Design = Item & { description: string | null; diagrams: string[] };
export type Project = {
  slug: string; description: string | null;
  counts: { issues: Record<string, number>; milestones: Record<string, number>; decisions: number; learnings: number };
  milestones: Milestone[]; loose: IssueRow[]; done: Milestone[]; designs: Design[];
  decisions: Described[]; learnings: Described[];
};
export type Linked = Item & { relation: string };
export type Artifact = Item & {
  description: string | null; goal: string | null; tags: string[]; html: string;
  toc: { depth: number; text: string; id: string }[];
  crumbs: { label: string; href: string }[];
  judge: { label: string; value: string; tone?: string; href?: string; mono?: boolean; title?: string }[];
  props: { k: string; v: string | string[] }[];
  diagrams: string[]; backlinks: Linked[]; forward: Linked[];
  graph: { nodes: Item[]; edges: { source: string; target: string; relation: string }[] };
  issues?: IssueRow[];
};
export type Stack = { key: string; title: string; layers: (Item & { bytes: number; html: string })[] };
export type List = { type: string; counts: Record<string, number>; projects: string[]; rows: (Item | IssueRow)[]; total: number };
export type Hit = Item & { snippet: string };
export type Entry = { key: string; type: string; title: string; status: string | null; href: string; project: string | null; updated: string | null };
export type Knowledge = {
  rows: Described[];
  topics: { slug: string; decisions: number; threads: number; open: number; moved: string }[];
  counts: Record<string, Record<string, number>>;
};
export type Home = { projects: ProjectSummary[]; milestones: Milestone[]; loose: IssueRow[]; recent: Described[] };

async function get<T>(path: string): Promise<T> {
  const r = await fetch(`/api${path}`);
  if (r.status === 404) throw new NotFound(path);
  if (!r.ok) throw new Error(`${path}: ${r.status}`);
  return r.json();
}
export class NotFound extends Error {}

const q = <T,>(key: unknown[], path: string, enabled = true) =>
  useQuery({ queryKey: key, queryFn: () => get<T>(path), enabled, refetchInterval: 15000 });

export const useNav = () => q<{ projects: ProjectSummary[] }>(["nav"], "/nav");
export const useHome = () => q<Home>(["home"], "/home");
export const useProject = (slug: string) => q<Project>(["project", slug], `/project/${slug}`);
export const useArtifact = (key: string) => q<Artifact>(["artifact", key], `/artifact/${encodeURIComponent(key)}`);
export const useStack = (key: string) => q<Stack>(["stack", key], `/stack/${encodeURIComponent(key)}`);
export const useList = (type: string, params: string) => q<List>(["list", type, params], `/list/${type}?${params}`);
export const useSearch = (term: string) => q<{ q: string; hits: Hit[] }>(["search", term], `/search?q=${encodeURIComponent(term)}`, !!term);
export const useKnowledge = (type: string) => q<Knowledge>(["knowledge", type], `/knowledge${type ? `?type=${type}` : ""}`);
export const usePalette = (enabled: boolean) =>
  useQuery({ queryKey: ["palette"], queryFn: () => get<Entry[]>("/palette"), enabled, staleTime: 30000 });

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
  if (days === 1) return "1d";
  if (days < 30) return `${days}d`;
  if (days < 365) return `${Math.round(days / 30)}mo`;
  return `${Math.round(days / 365)}y`;
}

export const typeWord = (t: string) => t.replace("-", " ");
