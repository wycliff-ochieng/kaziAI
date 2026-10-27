// REST helper for gateway endpoints.
// Authentication travels in the HttpOnly session cookie; no tenant header is
// trusted by the browser client.

export function gatewayHttpBase(): string {
  if (typeof window !== "undefined") {
    return "/api/v1";
  }
  return process.env.NEXT_PUBLIC_GATEWAY_HTTP || "http://localhost:3001/api/v1";
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${gatewayHttpBase()}${path}`, {
    method,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const payload = await res.json();
      message = payload?.error || payload?.message || message;
    } catch {
      const text = await res.text();
      if (text) message = text;
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const apiGet = <T,>(path: string) => request<T>("GET", path);
export const apiPost = <T,>(path: string, body?: unknown) => request<T>("POST", path, body);
export const apiPatch = <T,>(path: string, body?: unknown) => request<T>("PATCH", path, body);
export const apiDelete = <T,>(path: string) => request<T>("DELETE", path);

export function exportUrl(path: string): string {
  return `${gatewayHttpBase()}${path}`;
}
