import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRootRoute, createRoute, createRouter, RouterProvider } from "@tanstack/react-router";
import "./styles.css";
import { Shell } from "./components/Shell";
import { HomePage } from "./routes/Home";
import { ProjectPage } from "./routes/Project";
import { ArtifactPage } from "./routes/Artifact";
import { StackPage } from "./routes/Stack";
import { ListPage } from "./routes/List";
import { SearchPage } from "./routes/Search";
import { KnowledgePage } from "./routes/Knowledge";
import { DiagramPage } from "./routes/Diagram";

const rootRoute = createRootRoute({ component: Shell, notFoundComponent: () => <div className="p-10 text-dim">Nothing here.</div> });
const routes = [
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: HomePage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/p/$slug", component: ProjectPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/a/$key", component: ArtifactPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/a/$key/stack", component: StackPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/t/$type", component: ListPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/search", component: SearchPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/knowledge", component: KnowledgePage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/diagram/$name", component: DiagramPage }),
];

const router = createRouter({ routeTree: rootRoute.addChildren(routes), defaultPreload: "intent", scrollRestoration: true });
declare module "@tanstack/react-router" {
  interface Register { router: typeof router }
}

const client = new QueryClient({ defaultOptions: { queries: { staleTime: 5000, retry: (n, e) => !(e as Error).name.includes("NotFound") && n < 2 } } });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
