import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { serveStatic } from "@hono/node-server/serve-static";
import { readFileSync } from "node:fs";
import { api, diagrams } from "./api.ts";

const app = new Hono();

// Read-only by construction: any method but GET and HEAD is refused.
app.use("*", async (c, next) => {
  if (c.req.method !== "GET" && c.req.method !== "HEAD") return c.text("method not allowed", 405);
  await next();
});
app.route("/api", api);
app.route("/diagram-src", diagrams);

if (process.env.NODE_ENV === "production") {
  app.use("/assets/*", serveStatic({ root: "./dist" }));
  const index = readFileSync("./dist/index.html", "utf8");
  app.get("*", (c) => c.html(index));
}

const port = Number(process.env.PORT ?? (process.env.NODE_ENV === "production" ? 7790 : 7791));
serve({ fetch: app.fetch, hostname: "127.0.0.1", port }, (i) => console.error(`anvil web api on http://127.0.0.1:${i.port}`));
