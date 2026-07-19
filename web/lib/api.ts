const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface RequestOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
  token?: string;
}

/**
 * Thin fetch wrapper that:
 * - Prepends the API base URL
 * - JSON-encodes the body
 * - Attaches the JWT Authorization header if a token is provided
 */
export async function api<T = unknown>(
  path: string,
  opts: RequestOptions = {}
): Promise<T> {
  const { body, token, ...rest } = opts;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((rest.headers as Record<string, string>) || {}),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...rest,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  const data = await res.json();

  if (!res.ok) {
    throw new ApiError(res.status, data.error || "Unknown error");
  }

  return data as T;
}

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}
