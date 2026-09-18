// REST helper for gateway endpoints (audit, connectors).
// Tenant travels via X-Tenant-ID header, matching TenantAuth middleware.

export function gatewayHttpBase(): string {
  if (typeof window !== "undefined") {
    return "/api/v1";
  }
  return process.env.NEXT_PUBLIC_GATEWAY_HTTP || "http://localhost:3001/api/v1";
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${gatewayHttpBase()}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Tenant-ID": "default_tenant",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const apiGet = <T,>(path: string) => request<T>("GET", path);
export const apiPost = <T,>(path: string, body?: unknown) => request<T>("POST", path, body);
export const apiDelete = <T,>(path: string) => request<T>("DELETE", path);

export function exportUrl(path: string): string {
  return `${gatewayHttpBase()}${path}`;
}
