import { serve } from "@hono/node-server";
import { serveStatic } from "@hono/node-server/serve-static";
import { readFileSync } from "node:fs";
import { app } from "./app.ts";

const production = process.env.NODE_ENV === "production";

if (production) {
  app.use("/assets/*", serveStatic({ root: "./dist" }));
  const index = readFileSync("./dist/index.html", "utf8");
  // Bodies render as HTML; a script-src of 'self' stops any markup that slips
  // past the sanitiser from running.
  app.get("*", (c) => {
    c.header("Content-Security-Policy", "script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'");
    return c.html(index);
  });
}

const port = Number(process.env.PORT ?? (production ? 7790 : 7791));
serve({ fetch: app.fetch, hostname: "127.0.0.1", port }, (i) => console.error(`anvil web on http://127.0.0.1:${i.port}`));
