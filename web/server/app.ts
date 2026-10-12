import { Hono } from "hono";
import { api, diagrams } from "./api.ts";

const loopback = new Set(["127.0.0.1", "localhost", "[::1]"]);

export const app = new Hono()
  // Read-only by construction: any method but GET and HEAD is refused.
  .use("*", async (c, next) => {
    if (c.req.method !== "GET" && c.req.method !== "HEAD") return c.text("method not allowed", 405);
    await next();
  })
  // The bind is loopback, but a DNS-rebinding page reaches loopback under its
  // own hostname; refusing any Host that is not loopback closes that door.
  .use("*", async (c, next) => {
    const host = (c.req.header("host") ?? "").replace(/:\d+$/, "");
    if (!loopback.has(host)) return c.text("forbidden host", 403);
    await next();
  })
  .route("/api", api)
  .route("/diagram-src", diagrams);
