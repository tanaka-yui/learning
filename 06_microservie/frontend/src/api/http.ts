export class ApiError extends Error {
  code: string;
  traceId: string;
  constructor(code: string, message: string, traceId: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.traceId = traceId;
  }
}

export interface ApiResult<T> {
  data: T;
  traceId: string;
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<ApiResult<T>> {
  const base = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';
  const res = await fetch(base + path, { credentials: 'include', ...init });
  const headerTraceId = res.headers.get('X-Trace-Id') ?? '';

  if (!res.ok) {
    let body: { code?: string; message?: string; trace_id?: string } = {};
    try {
      body = await res.json();
    } catch {
      // ignore parse errors
    }
    throw new ApiError(
      body.code ?? 'UNKNOWN',
      body.message ?? res.statusText,
      body.trace_id ?? headerTraceId,
    );
  }

  if (res.status === 204) {
    return { data: undefined as T, traceId: headerTraceId };
  }
  const data = (await res.json()) as T;
  return { data, traceId: headerTraceId };
}
