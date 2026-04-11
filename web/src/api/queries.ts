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
} from "./types";

export const authQueries = {
  me: () =>
    queryOptions({
      queryKey: ["auth", "me"],
      queryFn: () => api.get<UserInfo>("/auth/me"),
      retry: false,
    }),
};

export const targetQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["targets", namespace],
      queryFn: () =>
        api.get<ResourceList<Target>>(`/namespaces/${namespace}/targets`),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["targets", namespace, name],
      queryFn: () =>
        api.get<Target>(`/namespaces/${namespace}/targets/${name}`),
    }),
};

export const releaseQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["releases", namespace],
      queryFn: () =>
        api.get<ResourceList<Release>>(`/namespaces/${namespace}/releases`),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["releases", namespace, name],
      queryFn: () =>
        api.get<Release>(`/namespaces/${namespace}/releases/${name}`),
    }),
};

export const releaseBindingQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["releasebindings", namespace],
      queryFn: () =>
        api.get<ResourceList<ReleaseBinding>>(
          `/namespaces/${namespace}/releasebindings`,
        ),
    }),
};

export const componentQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["components", namespace],
      queryFn: () =>
        api.get<ResourceList<Component>>(`/namespaces/${namespace}/components`),
    }),
  detail: (namespace: string, name: string) =>
    queryOptions({
      queryKey: ["components", namespace, name],
      queryFn: () =>
        api.get<Component>(`/namespaces/${namespace}/components/${name}`),
    }),
};

export const componentVersionQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["componentversions", namespace],
      queryFn: () =>
        api.get<ResourceList<ComponentVersion>>(
          `/namespaces/${namespace}/componentversions`,
        ),
    }),
};

export const registryQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["registries", namespace],
      queryFn: () =>
        api.get<ResourceList<Registry>>(`/namespaces/${namespace}/registries`),
    }),
};

export const profileQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["profiles", namespace],
      queryFn: () =>
        api.get<ResourceList<Profile>>(`/namespaces/${namespace}/profiles`),
    }),
};

export const renderTaskQueries = {
  list: (namespace: string) =>
    queryOptions({
      queryKey: ["rendertasks", namespace],
      queryFn: () =>
        api.get<ResourceList<RenderTask>>(
          `/namespaces/${namespace}/rendertasks`,
        ),
    }),
};
