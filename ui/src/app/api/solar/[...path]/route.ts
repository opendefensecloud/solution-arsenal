import { NextRequest, NextResponse } from "next/server";

const SOLAR_API_GROUP = "solar.opendefense.cloud";
const SOLAR_API_VERSION = "v1alpha1";

function getBaseURL(): string {
  if (process.env.SOLAR_API_URL) {
    return process.env.SOLAR_API_URL;
  }
  const host = process.env.KUBERNETES_SERVICE_HOST ?? "kubernetes.default.svc";
  const port = process.env.KUBERNETES_SERVICE_PORT ?? "443";
  return `https://${host}:${port}`;
}

function buildHeaders(): HeadersInit {
  const headers: HeadersInit = {
    Accept: "application/json",
  };
  // In-cluster: use mounted service account token
  if (!process.env.SOLAR_API_URL) {
    try {
      const fs = require("fs");
      const token = fs
        .readFileSync(
          "/var/run/secrets/kubernetes.io/serviceaccount/token",
          "utf8",
        )
        .trim();
      headers["Authorization"] = `Bearer ${token}`;
    } catch {
      // token not available
    }
  }
  return headers;
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const apiPath = `/apis/${SOLAR_API_GROUP}/${SOLAR_API_VERSION}/${path.join("/")}`;
  const url = `${getBaseURL()}${apiPath}`;

  const resp = await fetch(url, {
    headers: buildHeaders(),
  });

  const data = await resp.json();
  return NextResponse.json(data, { status: resp.status });
}
