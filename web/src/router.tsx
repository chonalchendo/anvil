import { createRootRoute, createRoute } from "@tanstack/react-router";
import { Shell } from "./components/Shell";
import { HomePage } from "./routes/Home";
import { ProjectPage } from "./routes/Project";
import { ArtifactPage } from "./routes/Artifact";
import { StackPage } from "./routes/Stack";
import { ListPage } from "./routes/List";
import { SearchPage } from "./routes/Search";
import { KnowledgePage } from "./routes/Knowledge";
import { DiagramPage } from "./routes/Diagram";

const str = (v: unknown) => (typeof v === "string" && v ? v : undefined);

const rootRoute = createRootRoute({ component: Shell, notFoundComponent: () => <div className="p-10 text-dim">Nothing here.</div> });
const routes = [
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: HomePage }),
  createRoute({
    getParentRoute: () => rootRoute, path: "/p/$slug", component: ProjectPage,
    validateSearch: (s): { view?: "board" } => (s.view === "board" ? { view: "board" } : {}),
  }),
  createRoute({ getParentRoute: () => rootRoute, path: "/a/$key", component: ArtifactPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/a/$key/stack", component: StackPage }),
  createRoute({
    getParentRoute: () => rootRoute, path: "/t/$type", component: ListPage,
    validateSearch: (s): { project?: string; status?: string; topic?: string } => ({ project: str(s.project), status: str(s.status), topic: str(s.topic) }),
  }),
  createRoute({
    getParentRoute: () => rootRoute, path: "/search", component: SearchPage,
    validateSearch: (s): { q?: string } => ({ q: str(s.q) }),
  }),
  createRoute({
    getParentRoute: () => rootRoute, path: "/knowledge", component: KnowledgePage,
    validateSearch: (s): { type?: string } => ({ type: str(s.type) }),
  }),
  createRoute({ getParentRoute: () => rootRoute, path: "/diagram/$name", component: DiagramPage }),
];

export const routeTree = rootRoute.addChildren(routes);
