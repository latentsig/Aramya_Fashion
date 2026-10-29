import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '../features/auth/AuthContext';
import { OrderDetailPage, AccountOrdersPage } from './OrdersPages';

const customer = { id: '123e4567-e89b-12d3-a456-426614174000', email: 'customer@example.com', displayName: 'Aramya Customer' };
const order = {
  id: '223e4567-e89b-12d3-a456-426614174000',
  orderNumber: 'AR-261029-TEST',
  status: 'pending_payment',
  currency: 'INR',
  subtotalPaise: 1250000,
  taxPaise: 0,
  shippingPaise: 0,
  discountPaise: 0,
  totalPaise: 1250000,
  createdAt: '2026-10-29T10:00:00Z',
  shippingAddress: { recipientName: 'Aramya Customer', phone: '9876543210', line1: '12 Silk Street', line2: '', city: 'Ankola', region: 'Karnataka', postalCode: '581314', countryCode: 'IN' },
  customerNote: 'Please call before delivery.',
  items: [{ id: '323e4567-e89b-12d3-a456-426614174000', productId: '423e4567-e89b-12d3-a456-426614174000', variantId: '523e4567-e89b-12d3-a456-426614174000', productName: 'Rose Gold Lehenga', productSku: 'BR-004', variantDescription: 'Custom size', quantity: 1, unitPricePaise: 1250000, taxPaise: 0, lineTotalPaise: 1250000 }],
};

function jsonResponse(status: number, payload: unknown) {
  return { ok: status >= 200 && status < 300, status, json: async () => payload } as Response;
}

function renderOrders(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter initialEntries={[path]}><Routes>
    <Route path="/account/orders" element={<AccountOrdersPage />} />
    <Route path="/account/orders/:id" element={<OrderDetailPage />} />
    <Route path="/login" element={<p>Sign in to continue</p>} />
  </Routes></MemoryRouter></AuthProvider></QueryClientProvider>);
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('customer order pages', () => {
  it('lists only the authenticated customer response and links to order details', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/auth/me')) return jsonResponse(200, { user: customer });
      if (url.includes('/orders?')) return jsonResponse(200, { data: [order], pagination: { limit: 20, offset: 0, count: 1, hasMore: false } });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    renderOrders('/account/orders');

    expect(await screen.findByText(/AR-261029-TEST/)).toBeTruthy();
    expect(screen.getByText('Pending Payment')).toBeTruthy();
    expect(screen.getByRole('link', { name: /view details/i }).getAttribute('href')).toBe(`/account/orders/${order.id}`);
    const ordersRequest = fetchMock.mock.calls.find(([input]) => String(input).includes('/orders?'));
    expect(ordersRequest?.[1]).toMatchObject({ credentials: 'include' });
  });

  it('renders the saved order snapshot and shipping details', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/auth/me')) return jsonResponse(200, { user: customer });
      if (url.endsWith(`/orders/${order.id}`)) return jsonResponse(200, { data: order });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const { container } = renderOrders(`/account/orders/${order.id}`);

    expect(await screen.findByRole('heading', { name: 'Rose Gold Lehenga' })).toBeTruthy();
    expect(container.querySelector('address')?.textContent).toContain('12 Silk Street');
    expect(screen.getByText('Please call before delivery.')).toBeTruthy();
    expect(screen.getByText(/payment is not enabled/i)).toBeTruthy();
  });

  it('redirects unauthenticated visitors to sign in', async () => {
    const fetchMock = vi.fn(async (_input?: RequestInfo | URL, _init?: RequestInit) => jsonResponse(401, { error: { code: 'unauthenticated' } }));
    vi.stubGlobal('fetch', fetchMock);

    renderOrders('/account/orders');

    expect(await screen.findByText('Sign in to continue')).toBeTruthy();
    expect(fetchMock.mock.calls.some(([input]) => String(input).includes('/orders'))).toBe(false);
  });
});
