import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminCategoriesPage, AdminProductsPage } from './AdminCatalogPages';

const category = {
  id: 'fd5a5719-8447-4a0b-9d7b-bcf609e00ad1', slug: 'sarees', name: 'Sarees', subtitle: 'A drape for every celebration',
  imageKey: 'image assests/saree1.jpg', sortOrder: 1, active: true, version: 1, activeProductCount: 39, updatedAt: '2026-10-29T10:00:00Z',
};
const image = { imageKey: 'image assests/saree1.jpg', label: 'saree1.jpg' };
const product = {
  id: '709991d6-a2be-4916-a4ce-7a79a18387a6', version: 1, sku: 'AR-TEST-01', slug: 'test-saree', name: 'Test Saree', description: 'A test garment',
  categoryId: category.id, categoryName: 'Sarees', pricePaise: 125050, currency: 'INR', imageKey: image.imageKey, status: 'draft', inventoryPolicy: 'made_to_order',
  defaultVariantId: '2b4172b4-2897-4aaa-aa58-9196176f6f03', updatedAt: '2026-10-29T10:00:00Z',
  variants: [{ id: '2b4172b4-2897-4aaa-aa58-9196176f6f03', sku: 'AR-TEST-01-MTO', optionName: 'Fit', optionValue: 'Made to measure', priceAdjustmentPaise: 0, active: true, isDefault: true }],
};
function response(data: unknown) { return new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } }); }
function page(data: unknown[]) { return response({ data, pagination: { limit: 50, offset: 0, count: data.length, hasMore: false } }); }

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('admin catalog pages', () => {
  it('creates a product from real image options and sends paise with CSRF protection', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/admin/categories')) return response({ data: [category] });
      if (url.endsWith('/admin/catalog/image-options')) return response({ data: [image] });
      if (url.includes('/admin/products?')) return page([]);
      if (url.endsWith('/auth/csrf')) return response({ csrfToken: 'csrf-token' });
      if (url.endsWith('/admin/products') && init?.method === 'POST') return new Response(JSON.stringify({ data: product }), { status: 201, headers: { 'Content-Type': 'application/json' } });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminProductsPage /></MemoryRouter>);
    fireEvent.click(await screen.findByRole('button', { name: /add product/i }));
    fireEvent.change(screen.getByLabelText('Product name'), { target: { value: 'Test Saree' } });
    fireEvent.change(screen.getByLabelText('URL slug'), { target: { value: 'test-saree' } });
    fireEvent.change(screen.getByLabelText('Product SKU'), { target: { value: 'AR-TEST-01' } });
    const categorySelect = screen.getByRole('dialog').querySelector('select[required]');
    if (!categorySelect) throw new Error('Product category selector is missing.');
    fireEvent.change(categorySelect, { target: { value: category.id } });
    fireEvent.change(screen.getByLabelText('Price (₹)'), { target: { value: '1250.50' } });
    fireEvent.change(screen.getByLabelText('Primary catalog image'), { target: { value: image.imageKey } });
    fireEvent.change(screen.getByLabelText('Variant SKU'), { target: { value: 'AR-TEST-01-MTO' } });
    fireEvent.change(screen.getByLabelText('Option value'), { target: { value: 'Made to measure' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create product' }));

    await waitFor(() => expect(fetcher).toHaveBeenCalledWith(expect.stringContaining('/admin/products'), expect.objectContaining({ method: 'POST' })));
    const call = fetcher.mock.calls.find(([url, init]) => String(url).endsWith('/admin/products') && init?.method === 'POST');
    expect(JSON.parse(String(call?.[1]?.body))).toMatchObject({ pricePaise: 125050, categoryId: category.id, imageKey: image.imageKey });
    expect((call?.[1]?.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-token');
    expect((await screen.findByRole('status')).textContent).toContain('Product created.');
  });

  it('shows active-product context while editing a category', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/admin/categories')) return response({ data: [category] });
      if (url.endsWith('/admin/catalog/image-options')) return response({ data: [image] });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminCategoriesPage /></MemoryRouter>);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit category' }));
    expect(await screen.findByText(/archive or move the 39 active products/i)).toBeTruthy();
    expect((screen.getByRole('checkbox', { name: 'Collection is visible' }) as HTMLInputElement).disabled).toBe(true);
  });
});
