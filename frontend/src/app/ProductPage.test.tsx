import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ProductPage } from './ProductPage';
import { CartProvider } from '../features/cart/CartContext';

const productId = '8cc755ef-de88-5e6b-8e46-79bba27921cc';
const variantId = '7167d1c8-7429-5aaa-9a7e-bd1822f59479';

describe('product detail page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('loads the live variant and adds that selected variant to the persistent cart', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/api/v1/products/the-heirloom-saree')) return new Response(JSON.stringify({ data: {
        id: productId, defaultVariantId: variantId, sku: 'SAREES-001', slug: 'the-heirloom-saree', name: 'The Heirloom Saree',
        description: null, categoryId: 'c03bec7c-5ef9-57e9-b27d-69a1e642bcaf', categorySlug: 'sarees', categoryName: 'Sarees',
        pricePaise: 890000, currency: 'INR', imageKey: 'image assests/saree1.jpg', status: 'active', inventoryPolicy: 'made_to_order',
        variants: [{ id: variantId, sku: 'SAREES-001-MTO', optionName: 'Fit', optionValue: 'Made to measure', priceAdjustmentPaise: 0, isDefault: true, available: true }],
        images: [{ imageKey: 'image assests/saree1.jpg', altText: 'The Heirloom Saree' }],
      } }), { status: 200 });
      if (url.includes('/api/v1/auth/csrf')) return new Response(JSON.stringify({ csrfToken: 'test-signed-csrf' }), { status: 200 });
      if (url.endsWith('/api/v1/cart') && (init?.method ?? 'GET') === 'GET') return new Response(JSON.stringify({ data: { lines: [], count: 0, subtotalPaise: 0 } }), { status: 200 });
      if (url.endsWith('/api/v1/cart/items') && init?.method === 'POST') return new Response(JSON.stringify({ data: {
        lines: [{ id: 'c042ea04-6279-4533-8c48-8dc5f7ab16f9', variantId, productId, slug: 'the-heirloom-saree', name: 'The Heirloom Saree', category: 'sarees', categoryName: 'Sarees', imageKey: 'image assests/saree1.jpg', unitPricePaise: 890000, quantity: 1 }],
        count: 1, subtotalPaise: 890000,
      } }), { status: 200 });
      return new Response(JSON.stringify({ error: { message: `Unexpected request: ${url}` } }), { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    render(<QueryClientProvider client={queryClient}><MemoryRouter initialEntries={['/products/the-heirloom-saree']}><CartProvider><Routes><Route path="/products/:slug" element={<ProductPage />} /></Routes></CartProvider></MemoryRouter></QueryClientProvider>);

    expect(await screen.findByRole('heading', { name: 'The Heirloom Saree' })).toBeTruthy();
    expect(screen.getByRole('radio', { name: 'Made to measure' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /add to bag/i }));
    expect(await screen.findByText('Added to your bag.')).toBeTruthy();

    const addRequest = fetchMock.mock.calls.find(([input, init]) => String(input).endsWith('/api/v1/cart/items') && init?.method === 'POST');
    expect(addRequest).toBeTruthy();
    expect(JSON.parse(String(addRequest?.[1]?.body))).toEqual({ variantId, quantity: 1 });
    await waitFor(() => expect(screen.getByRole('link', { name: 'View bag ↗' })).toBeTruthy());
  });
});
