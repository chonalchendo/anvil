import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import "./styles.css";
import { NotFound } from "./lib/api";
import { routeTree } from "./router";

const router = createRouter({ routeTree, defaultPreload: "intent", scrollRestoration: true });
declare module "@tanstack/react-router" {
  interface Register { router: typeof router }
}

const client = new QueryClient({ defaultOptions: { queries: { staleTime: 5000, retry: (n, e) => !(e instanceof NotFound) && n < 2 } } });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
