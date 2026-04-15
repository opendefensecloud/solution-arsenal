// Kubernetes-style metadata
export interface ObjectMeta {
  name: string;
  namespace: string;
  creationTimestamp: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  generation?: number;
}

// Condition from K8s status
export interface Condition {
  type: string;
  status: "True" | "False" | "Unknown";
  lastTransitionTime: string;
  reason: string;
  message: string;
}

// Target
export interface Target {
  metadata: ObjectMeta;
  spec: {
    renderRegistryRef: { name: string };
    userdata?: unknown;
  };
  status?: {
    conditions?: Condition[];
  };
}

// Release
export interface Release {
  metadata: ObjectMeta;
  spec: {
    componentVersionRef: { name: string };
  };
  status?: {
    conditions?: Condition[];
  };
}

// ReleaseBinding
export interface ReleaseBinding {
  metadata: ObjectMeta;
  spec: {
    targetRef: { name: string };
    releaseRef: { name: string };
  };
  status?: {
    conditions?: Condition[];
  };
}

// Component
export interface Component {
  metadata: ObjectMeta;
  spec: {
    scheme: string;
    repository: string;
    registry: string;
  };
}

// ComponentVersion
export interface ComponentVersion {
  metadata: ObjectMeta;
  spec: {
    componentRef: { name: string };
    tag: string;
    resources?: Record<
      string,
      {
        repository: string;
        tag: string;
        insecure?: boolean;
      }
    >;
    entrypoint?: {
      type: string;
      resourceName: string;
    };
  };
}

// Registry
export interface Registry {
  metadata: ObjectMeta;
  spec: {
    hostname: string;
    plainHTTP?: boolean;
    solarSecretRef?: { name: string };
    targetSecretRef?: { name: string; namespace: string };
  };
  status?: {
    conditions?: Condition[];
  };
}

// Profile
export interface Profile {
  metadata: ObjectMeta;
  spec: {
    releaseRef: { name: string };
    targetSelector: {
      matchLabels?: Record<string, string>;
    };
  };
  status?: {
    conditions?: Condition[];
    matchedTargets?: number;
  };
}

// RenderTask
export interface RenderTask {
  metadata: ObjectMeta;
  spec: {
    type: string;
    baseURL: string;
  };
  status?: {
    conditions?: Condition[];
  };
}

// List wrapper
export interface ResourceList<T> {
  items: T[];
}

// Auth
export interface UserInfo {
  username: string;
  groups: string[];
  authenticated: boolean;
  impersonating?: {
    username: string;
    groups: string[];
  };
}

// SSE event
export interface ResourceEvent {
  type: "ADDED" | "MODIFIED" | "DELETED";
  resource: string;
  namespace: string;
  name: string;
}
