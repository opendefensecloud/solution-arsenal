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
import { ComponentVersionsPage } from "@/pages/components/versions";
import { ProfilesPage } from "@/pages/profiles";
import { RegistriesPage } from "@/pages/registries";

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

const componentVersionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/components/$namespace/$name",
  component: ComponentVersionsPage,
});

const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profiles",
  component: ProfilesPage,
});

const registriesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/registries",
  component: RegistriesPage,
});

export const routeTree = rootRoute.addChildren([
  dashboardRoute,
  targetsRoute,
  releasesRoute,
  componentsRoute,
  componentVersionsRoute,
  profilesRoute,
  registriesRoute,
]);
