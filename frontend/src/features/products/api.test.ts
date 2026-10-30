import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchCatalogProducts } from './api';
import { imageUrl } from './catalog';

describe('fetchCatalogProducts', () => {
  afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

  it('maps the database API response to the storefront product model', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [{
      id: '7af7c9e8-8c8d-5ad5-b48e-51bb0fe40b5c', defaultVariantId: '7af7c9e8-8c8d-5ad5-b48e-51bb0fe40b5d', sku: 'SAREES-001', slug: 'heirloom-saree', name: 'The Heirloom Saree',
      description: null, categoryId: '6bc7e5d2-284e-5982-a93a-c44c401f99a8', categorySlug: 'sarees', categoryName: 'Sarees',
      pricePaise: 890000, currency: 'INR', imageKey: 'image assests/saree1.jpg', status: 'active', inventoryPolicy: 'made_to_order',
    }] }), { status: 200 })));
    const products = await fetchCatalogProducts('sarees');
    expect(products).toHaveLength(1);
    expect(products[0]).toMatchObject({ name: 'The Heirloom Saree', category: 'sarees', image: 'saree1.jpg', priceRupees: 8900, sku: 'SAREES-001' });
  });

  it('rejects unsuccessful API responses', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 503 })));
    await expect(fetchCatalogProducts()).rejects.toThrow('Catalog request failed (503)');
  });

  it('resolves approved object keys to the configured media origin without flattening the path', () => {
    vi.stubEnv('VITE_MEDIA_BASE_URL', 'https://media.example.test/');
    expect(imageUrl('products/sarees/a1b2c3.webp')).toBe('https://media.example.test/products/sarees/a1b2c3.webp');
    expect(imageUrl('image assests/saree1.jpg')).toBe('/media/saree1.jpg');
  });
});
