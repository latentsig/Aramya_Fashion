import { afterEach, describe, expect, it, vi } from 'vitest';
import { uploadCatalogImage } from './catalogAdminApi';

afterEach(() => vi.unstubAllGlobals());

describe('uploadCatalogImage', () => {
  it('hashes the selected file, sends it to private storage, and completes verification', async () => {
    const media = { id: 'ce62e45e-c891-4c5a-b9b4-847fc7ba92d9', objectKey: 'products/sarees/ce62e45e-c891-4c5a-b9b4-847fc7ba92d9.jpg', status: 'pending' };
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/auth/csrf')) return new Response(JSON.stringify({ csrfToken: 'csrf-token' }), { status: 200 });
      if (url.endsWith('/admin/media/upload-intent')) return new Response(JSON.stringify({ data: { media, uploadUrl: 'https://r2.example.test/signed', expiresAt: '2026-10-30T10:05:00Z', requiredHeaders: { 'Content-Type': 'image/jpeg' } } }), { status: 201 });
      if (url === 'https://r2.example.test/signed') {
        expect(init?.method).toBe('PUT');
        expect(init?.body).toBe(file);
        expect((init?.headers as Record<string, string>)['Content-Type']).toBe('image/jpeg');
        return new Response(null, { status: 200 });
      }
      if (url.endsWith('/admin/media/complete')) return new Response(JSON.stringify({ data: { ...media, status: 'approved', publicUrl: `https://media.example.test/${media.objectKey}` } }), { status: 200 });
      throw new Error(`Unexpected request ${url}`);
    });
    const file = new File(['genuine test image bytes'], 'saree.jpg', { type: 'image/jpeg' });
    Object.defineProperty(file, 'arrayBuffer', { value: async () => new TextEncoder().encode('genuine test image bytes').buffer });
    vi.stubGlobal('fetch', fetcher);
    vi.stubGlobal('crypto', { subtle: { digest: vi.fn().mockResolvedValue(new Uint8Array(32).buffer) } });

    await expect(uploadCatalogImage(file, 'sarees', 'Test saree')).resolves.toEqual({ imageKey: media.objectKey, label: 'saree.jpg' });
    const writes = fetcher.mock.calls.filter(([, init]) => init?.method === 'POST');
    expect(writes).toHaveLength(2);
    expect(writes.map(([, init]) => (init?.headers as Record<string, string>)['X-CSRF-Token'])).toEqual(['csrf-token', 'csrf-token']);
    expect(JSON.parse(String(writes[1]?.[1]?.body))).toEqual({ intentId: media.id });
  });

  it('surfaces unavailable R2 configuration instead of reporting success', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => String(input).endsWith('/auth/csrf')
      ? new Response(JSON.stringify({ csrfToken: 'csrf-token' }), { status: 200 })
      : new Response(JSON.stringify({ error: { message: 'Product media storage is not configured in this environment' } }), { status: 503 }));
    vi.stubGlobal('fetch', fetcher);
    vi.stubGlobal('crypto', { subtle: { digest: vi.fn().mockResolvedValue(new Uint8Array(32).buffer) } });
    const file = new File(['test'], 'saree.jpg', { type: 'image/jpeg' });
    Object.defineProperty(file, 'arrayBuffer', { value: async () => new TextEncoder().encode('test').buffer });
    await expect(uploadCatalogImage(file, 'sarees', 'Test saree')).rejects.toThrow('Product media storage is not configured');
  });

  it('rejects unsupported and oversized files before hashing or making requests', async () => {
    const digest = vi.fn();
    const fetcher = vi.fn();
    vi.stubGlobal('fetch', fetcher);
    vi.stubGlobal('crypto', { subtle: { digest } });
    const svg = new File(['<svg/>'], 'art.svg', { type: 'image/svg+xml' });
    const oversized = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'large.jpg', { type: 'image/jpeg' });
    await expect(uploadCatalogImage(svg, 'sarees', 'Artwork')).rejects.toThrow('JPEG, PNG, WebP or AVIF');
    await expect(uploadCatalogImage(oversized, 'sarees', 'Large')).rejects.toThrow('10 MB');
    expect(digest).not.toHaveBeenCalled();
    expect(fetcher).not.toHaveBeenCalled();
  });
});
