import { describe, expect, it } from "vitest";
import type { Entry } from "./api";
import { group, rank } from "./rank";

const e = (key: string, type: string, title: string, status: string | null = "open"): Entry =>
  ({ key, type, title, status, project: "demo", href: `/a/${key}`, updated: "2026-10-01" });

const entries = [
  e("issue.demo.0001.palette-ranking", "issue", "Rank the palette by relevance"),
  e("milestone.demo.palette", "milestone", "Palette"),
  e("decision.ui.0001", "decision", "Use a command palette"),
  e("demo", "project", "demo", null),
];

describe("rank", () => {
  it("puts a title or key-segment prefix before a word prefix", () => {
    expect(rank(entries, "palette").map((x) => x.key)).toEqual([
      "issue.demo.0001.palette-ranking", "milestone.demo.palette", "decision.ui.0001",
    ]);
  });

  it("requires every word to match", () => {
    expect(rank(entries, "command palette").map((x) => x.key)).toEqual(["decision.ui.0001"]);
  });

  it("offers projects and milestones with no term", () => {
    expect(rank(entries, "").map((x) => x.type)).toEqual(["milestone", "project"]);
  });
});

describe("group", () => {
  it("keeps rank order with a term and type order without", () => {
    const hits = rank(entries, "palette");
    expect(group(hits, "palette").map(([t]) => t)).toEqual(["issue", "milestone", "decision"]);
    expect(group(hits, "").map(([t]) => t)).toEqual(["milestone", "issue", "decision"]);
  });
});
