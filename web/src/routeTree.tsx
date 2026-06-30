import { createRootRouteWithContext, createRoute, Outlet } from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import { Layout } from '@/components/layout'
import { DashboardPage } from '@/pages/dashboard'
import { TargetsPage } from '@/pages/targets'
import { TargetDetailPage } from '@/pages/targets/detail'
import { ReleasesPage } from '@/pages/releases'
import { ReleaseDetailPage } from '@/pages/releases/detail'
import { ComponentsPage } from '@/pages/components'
import { ComponentVersionsPage } from '@/pages/components/versions'
import { ProfilesPage } from '@/pages/profiles'
import { ProfileDetailPage } from '@/pages/profiles/detail'
import { RegistriesPage } from '@/pages/registries'
import { RegistryDetailPage } from '@/pages/registries/detail'
import { PipelinePage } from '@/pages/pipeline'

interface RouterContext {
  queryClient: QueryClient
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <Layout>
      <Outlet />
    </Layout>
  ),
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
})

const targetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/targets',
  component: TargetsPage,
})

const targetDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/targets/$namespace/$name',
  component: TargetDetailPage,
})

const releasesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/releases',
  component: ReleasesPage,
})

const releaseDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/releases/$namespace/$name',
  component: ReleaseDetailPage,
})

const componentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/components',
  component: ComponentsPage,
})

const componentVersionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/components/$namespace/$name',
  component: ComponentVersionsPage,
})

const profilesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/profiles',
  component: ProfilesPage,
})

const profileDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/profiles/$namespace/$name',
  component: ProfileDetailPage,
})

const registriesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/registries',
  component: RegistriesPage,
})

const registryDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/registries/$namespace/$name',
  component: RegistryDetailPage,
})

const pipelineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pipeline',
  component: PipelinePage,
})

export const routeTree = rootRoute.addChildren([
  dashboardRoute,
  targetsRoute,
  targetDetailRoute,
  releasesRoute,
  releaseDetailRoute,
  componentsRoute,
  componentVersionsRoute,
  profilesRoute,
  profileDetailRoute,
  registriesRoute,
  registryDetailRoute,
  pipelineRoute,
])
