import {
  createRootRouteWithContext,
  createRoute,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { Layout } from "@/components/layout";
import { DashboardPage } from "@/pages/dashboard";
import { TargetsPage } from "@/pages/targets";
import { ReleasesPage } from "@/pages/releases";
import { ComponentsPage } from "@/pages/components";
import { ProfilesPage } from "@/pages/profiles";
import { permissionQueries } from "@/api/queries";
import { permissionsFromResponse } from "@/hooks/usePermissions";

interface RouterContext {
  queryClient: QueryClient;
}

// All guarded routes operate on this namespace.
// When a namespace selector is added this should come from router context.
const DEFAULT_NAMESPACE = "default";

const SOLAR_GROUP = "solar.opendefense.cloud";

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
  beforeLoad: async ({ context }) => {
    const data = await context.queryClient.ensureQueryData(
      permissionQueries.rules(DEFAULT_NAMESPACE),
    );
    const can = permissionsFromResponse(data);
    if (!can("list", "targets", SOLAR_GROUP)) {
      throw redirect({ to: "/" });
    }
  },
  component: TargetsPage,
});

const releasesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/releases",
  beforeLoad: async ({ context }) => {
    const data = await context.queryClient.ensureQueryData(
      permissionQueries.rules(DEFAULT_NAMESPACE),
    );
    const can = permissionsFromResponse(data);
    if (!can("list", "releases", SOLAR_GROUP)) {
      throw redirect({ to: "/" });
    }
  },
  component: ReleasesPage,
});

const componentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/components",
  beforeLoad: async ({ context }) => {
    const data = await context.queryClient.ensureQueryData(
      permissionQueries.rules(DEFAULT_NAMESPACE),
    );
    const can = permissionsFromResponse(data);
    if (!can("list", "components", SOLAR_GROUP)) {
      throw redirect({ to: "/" });
    }
  },
  component: ComponentsPage,
});

const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profiles",
  beforeLoad: async ({ context }) => {
    const data = await context.queryClient.ensureQueryData(
      permissionQueries.rules(DEFAULT_NAMESPACE),
    );
    const can = permissionsFromResponse(data);
    if (!can("list", "profiles", SOLAR_GROUP)) {
      throw redirect({ to: "/" });
    }
  },
  component: ProfilesPage,
});

export const routeTree = rootRoute.addChildren([
  dashboardRoute,
  targetsRoute,
  releasesRoute,
  componentsRoute,
  profilesRoute,
]);
