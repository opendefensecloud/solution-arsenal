import {
  createRootRouteWithContext,
  createRoute,
  Outlet,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { Layout } from "@/components/layout";
import { DashboardPage } from "@/pages/dashboard";
import { TargetsPage } from "@/pages/targets";
import { ReleasesPage } from "@/pages/releases";
import { ComponentsPage } from "@/pages/components";
import { ProfilesPage } from "@/pages/profiles";

interface RouterContext {
  queryClient: QueryClient;
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <Layout>
      <Outlet />
    </Layout>
  ),
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: DashboardPage,
});

const targetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/targets",
  component: TargetsPage,
});

const releasesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/releases",
  component: ReleasesPage,
});

const componentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/components",
  component: ComponentsPage,
});

const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profiles",
  component: ProfilesPage,
});

export const routeTree = rootRoute.addChildren([
  dashboardRoute,
  targetsRoute,
  releasesRoute,
  componentsRoute,
  profilesRoute,
]);
