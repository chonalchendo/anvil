// @vitest-environment node
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { beforeAll, describe, expect, it } from "vitest";
import { seedVault } from "./fixture.ts";

let root = "";
let app: typeof import("../app.ts").app;

// vault.ts opens the index at import time, so the vault must exist first.
beforeAll(async () => {
  root = seedVault();
  process.env.ANVIL_VAULT = root;
  ({ app } = await import("../app.ts"));
});

const get = (path: string, host = "127.0.0.1:7791") => app.request(path, { headers: { host } });
const json = async <T>(path: string) => (await (await get(path)).json()) as T;

function hashTree(dir: string): string {
  const h = createHash("sha256");
  const walk = (d: string) => {
    for (const name of readdirSync(d).sort()) {
      if (name === ".anvil") continue;
      const p = join(d, name);
      if (statSync(p).isDirectory()) walk(p);
      else h.update(p).update(readFileSync(p));
    }
  };
  walk(dir);
  return h.digest("hex");
}

describe("guards", () => {
  it("refuses writes", async () => {
    expect((await app.request("/api/home", { method: "POST", headers: { host: "localhost" } })).status).toBe(405);
  });

  it("refuses a foreign Host", async () => {
    expect((await get("/api/home", "evil.example")).status).toBe(403);
  });

  it("never writes to the vault", async () => {
    const before = hashTree(root);
    for (const p of ["/api/nav", "/api/home", "/api/project/demo", "/api/artifact/issue.demo.0001.live-issue",
      "/api/stack/issue.demo.0001.live-issue", "/api/list/issue", "/api/search?q=loose", "/api/palette", "/api/knowledge"])
      expect((await get(p)).status, p).toBe(200);
    expect(hashTree(root)).toBe(before);
  });
});

describe("markdown", () => {
  it("drops script URLs and escapes raw HTML", async () => {
    const a = await json<{ html: string }>("/api/artifact/issue.demo.0001.live-issue");
    expect(a.html).not.toMatch(/(href|src)="javascript:/i);
    expect(a.html).not.toContain("<script");
    expect(a.html).toContain("&#x3C;script>");
    expect(a.html).toContain('href="/a/decision.demo-topic.0001-a-decision"');
  });
});

describe("artifact", () => {
  it("keeps non-PR links in props and judge keys out of them", async () => {
    const a = await json<{ props: { k: string; v: string[] }[]; issues: unknown }>("/api/artifact/issue.demo.0001.live-issue");
    const keys = a.props.map((p) => p.k);
    expect(keys).not.toContain("verified_verdict");
    expect(a.props.find((p) => p.k === "external_links")?.v).toEqual(["https://example.com/notes"]);
    expect(a.issues).toBeNull();
  });
});

describe("stack", () => {
  it("loads layers in the hydrate manifest order", async () => {
    const s = await json<{ layers: { key: string }[] }>("/api/stack/issue.demo.0001.live-issue");
    const mentions = execFileSync("anvil", ["hydrate", "issue.demo.0001.live-issue", "--tldr"], { env: { ...process.env, ANVIL_VAULT: root } })
      .toString().match(/[\w-]+\.[\w.-]+/g) ?? [];
    const order = s.layers.map((l) => l.key);
    expect([...new Set(mentions)].filter((k) => order.includes(k))).toEqual(order);
  });

  it("reports a broken spine edge instead of failing", async () => {
    const res = await get("/api/stack/issue.demo.0003.broken-edge");
    expect(res.status).toBe(200);
    const s = (await res.json()) as { broken: string[] };
    expect(s.broken.join("\n")).toContain("milestone.demo.gone");
  });
});

describe("lists", () => {
  it("counts an escalated issue without a milestone as loose work", async () => {
    const h = await json<{ loose: { key: string }[] }>("/api/home");
    expect(h.loose.map((i) => i.key)).toContain("issue.demo.0002.escalated-loose");
  });

  it("refuses sessions on the knowledge page", async () => {
    expect((await get("/api/knowledge?type=session")).status).toBe(404);
  });

  it("survives a NUL in a search", async () => {
    expect((await get("/api/search?q=loose%00x")).status).toBe(200);
  });
});

describe("diagrams", () => {
  it("serves a diagram under a locked CSP", async () => {
    const res = await get("/diagram-src/demo-flow");
    expect(res.status).toBe(200);
    expect(res.headers.get("content-security-policy")).toBe("default-src 'none'; style-src 'unsafe-inline'; font-src 'self'; img-src data:");
  });

  it("refuses a diagram that holds a script", async () => {
    expect((await get("/diagram-src/demo-evil")).status).toBe(415);
  });
});
