// API client for solar-ui backend

const API_BASE = "/api";

export interface ApiError {
  status: number;
  message: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
    credentials: "same-origin",
  });

  if (res.status === 401) {
    // Redirect to OIDC login flow
    window.location.href = "/api/auth/login";

    // Return a never-resolving promise to prevent further rendering
    return new Promise<T>(() => {});
  }

  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw { status: res.status, message: body || res.statusText } as ApiError;
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: JSON.stringify(body) }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
