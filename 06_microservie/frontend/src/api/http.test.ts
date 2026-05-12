import { describe, it, expect, beforeEach, vi } from 'vitest';
import { apiFetch, ApiError } from './http';

describe('apiFetch', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
    vi.stubEnv('VITE_API_BASE', 'http://api.test');
  });

  it('returns data and traceId on success', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(JSON.stringify({ products: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json', 'X-Trace-Id': 'abc123' },
      }),
    );
    const { data, traceId } = await apiFetch<{ products: unknown[] }>('/api/products');
    expect(data.products).toEqual([]);
    expect(traceId).toBe('abc123');
  });

  it('returns undefined data on 204', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(null, { status: 204, headers: { 'X-Trace-Id': 'xyz' } }),
    );
    const { data, traceId } = await apiFetch<undefined>('/api/auth/signin', { method: 'POST' });
    expect(data).toBeUndefined();
    expect(traceId).toBe('xyz');
  });

  it('throws ApiError with code/message/traceId on non-2xx', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(JSON.stringify({ code: 'INVALID_INPUT', message: 'bad', trace_id: 'tr-1' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json', 'X-Trace-Id': 'tr-1' },
      }),
    );
    await expect(apiFetch('/api/whatever')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'INVALID_INPUT',
      message: 'bad',
      traceId: 'tr-1',
    });
  });

  it('thrown ApiError is instance of ApiError', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(JSON.stringify({ code: 'X', message: 'y', trace_id: 'z' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    try {
      await apiFetch('/api/whatever');
      throw new Error('should have thrown');
    } catch (e) {
      expect(e instanceof ApiError).toBe(true);
    }
  });
});
