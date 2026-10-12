import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { renderAt } from "./renderAt";

const server = setupServer(http.get("/api/nav", () => HttpResponse.json({ projects: [] })));
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const issue = {
  key: "issue.demo.0001.live", type: "issue", status: "in-progress", project: "demo", title: "Live issue",
  created: "2026-10-01", updated: "2026-10-03", ordinal: "0001", owner: "tester", severity: null,
  verdict: "pass", prs: [{ n: 7, url: "https://github.com/x/y/pull/7" }], rounds: 2, tokens: 1234567,
};

describe("pages", () => {
  it("names the failure when the home query errors", async () => {
    server.use(http.get("/api/home", () => HttpResponse.json({ error: "boom" }, { status: 500 })));
    renderAt("/");
    expect(await screen.findByRole("alert")).toHaveTextContent(/Could not load what is moving/);
  });

  it("says not found when knowledge 404s", async () => {
    server.use(http.get("/api/knowledge", () => HttpResponse.json({ error: "not found" }, { status: 404 })));
    renderAt("/knowledge?type=session");
    expect(await screen.findByRole("alert")).toHaveTextContent(/No knowledge here/);
  });

  it("shows the issue list with verdict, PR and compact tokens", async () => {
    server.use(http.get("/api/list/issue", () =>
      HttpResponse.json({ type: "issue", counts: { "in-progress": 1 }, projects: ["demo"], rows: [issue], issues: [issue], total: 1 }),
    ));
    renderAt("/t/issue?project=demo");
    const row = await screen.findByRole("row", { name: /Live issue/ });
    expect(row).toHaveTextContent("pass");
    expect(row).toHaveTextContent("#7");
    expect(row).toHaveTextContent("1.2M");
  });

  it("sends the status filter from the URL to the server", async () => {
    let seen = "";
    server.use(http.get("/api/list/issue", ({ request }) => {
      seen = new URL(request.url).search;
      return HttpResponse.json({ type: "issue", counts: {}, projects: [], rows: [], issues: [], total: 0 });
    }));
    renderAt("/t/issue?status=open");
    expect(await screen.findByText("Nothing matches.")).toBeInTheDocument();
    expect(seen).toBe("?status=open");
  });
});
