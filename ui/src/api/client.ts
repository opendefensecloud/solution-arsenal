import createClient from "openapi-fetch";
import type { paths, components } from "./solar";

/**
 * Typed SolAr API client built from the generated OpenAPI spec.
 */
const client = createClient<paths>({ baseUrl: "/api/solar" });

export default client;

// ---- Convenience type aliases for SolAr resources ----

export type Bootstrap =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Bootstrap"];
export type BootstrapList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.BootstrapList"];
export type Component =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Component"];
export type ComponentList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.ComponentList"];
export type ComponentVersion =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.ComponentVersion"];
export type ComponentVersionList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.ComponentVersionList"];
export type Discovery =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Discovery"];
export type DiscoveryList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.DiscoveryList"];
export type Profile =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Profile"];
export type ProfileList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.ProfileList"];
export type Release =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Release"];
export type ReleaseList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.ReleaseList"];
export type RenderTask =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.RenderTask"];
export type RenderTaskList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.RenderTaskList"];
export type Target =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.Target"];
export type TargetList =
  components["schemas"]["cloud.opendefense.solar.v1alpha1.TargetList"];
