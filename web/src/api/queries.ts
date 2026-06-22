import { queryOptions } from "@tanstack/react-query";
import { api } from "./client";
import type {
  Target,
  Release,
  ReleaseBinding,
  Component,
  ComponentVersion,
  Registry,
  Profile,
  RenderTask,
  ResourceList,
  UserInfo,
  Namespace,
} from "./types";

/**
 * A query namespace is either a specific namespace string, or null meaning
 * "all namespaces" (cluster-wide list). The BFF has matching routes:
 *   - GET /api/{resource}            → cluster-wide
 *   - GET /api/namespaces/{ns}/{r}   → namespace-scoped
 * RBAC is enforced server-side by K8s.
 */
export type QueryNamespace = string | null;

function nsPath(resource: string, namespace: QueryNamespace): string {
  return namespace === null
    ? `/${resource}`
    : `/namespaces/${namespace}/${resource}`;
}

function nsKey(namespace: QueryNamespace): string {
  return namespace ?? "*";
}

export const authQueries = {
  me: () =>
    queryOptions({
      queryKey: ["auth", "me"],
      queryFn: () => api.get<UserInfo>("/auth/me"),
      retry: false,
    }),
};

export const namespaceQueries = {
  list: () =>
    queryOptions({
      queryKey: ["namespaces"],
      queryFn: () => api.get<ResourceList<Namespace>>("/namespaces"),
      retry: false,
    }),
};

export const targetQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["targets", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<Target>>(nsPath("targets", namespace)),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["targets", namespace, name],
      queryFn: () =>
        api.get<Target>(`/namespaces/${namespace}/targets/${name}`),
    }),
};

export const releaseQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["releases", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<Release>>(nsPath("releases", namespace)),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["releases", namespace, name],
      queryFn: () =>
        api.get<Release>(`/namespaces/${namespace}/releases/${name}`),
    }),
};

export const releaseBindingQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["releasebindings", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<ReleaseBinding>>(nsPath("releasebindings", namespace)),
    }),
};

export const componentQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["components", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<Component>>(nsPath("components", namespace)),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["components", namespace, name],
      queryFn: () =>
        api.get<Component>(`/namespaces/${namespace}/components/${name}`),
    }),
};

export const componentVersionQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["componentversions", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<ComponentVersion>>(nsPath("componentversions", namespace)),
    }),
};

export const registryQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["registries", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<Registry>>(nsPath("registries", namespace)),
    }),
};

export const profileQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["profiles", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<Profile>>(nsPath("profiles", namespace)),
    }),
};

export const renderTaskQueries = {
  list: (namespace: QueryNamespace) =>
    queryOptions({
      queryKey: ["rendertasks", nsKey(namespace)],
      queryFn: () =>
        api.get<ResourceList<RenderTask>>(nsPath("rendertasks", namespace)),
    }),
};
