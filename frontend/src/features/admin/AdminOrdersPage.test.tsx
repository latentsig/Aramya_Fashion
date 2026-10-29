import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminOrdersPage } from './AdminOrdersPage';

const order = {
  id: '2b4172b4-2897-4aaa-aa58-9196176f6f03', version: 4, orderNumber: 'AR-2026-001', status: 'ready_to_ship', currency: 'INR',
  subtotalPaise: 200000, taxPaise: 0, shippingPaise: 0, discountPaise: 0, totalPaise: 200000, customerName: 'Test Customer',
  customerEmail: 'customer@example.test', customerPhone: '9876500000', paymentStatus: 'captured', refundStatus: '', refundedPaise: 0, shipmentStatus: 'ready', shipmentProvider: '',
  trackingNumber: '', trackingUrl: '', createdAt: '2026-10-29T10:00:00Z', updatedAt: '2026-10-29T10:30:00Z',
};
const detail = {
  ...order, shippingAddress: { recipientName: 'Test Customer', phone: '9876500000', line1: '12 Silk Street', city: 'Ankola', region: 'Karnataka', postalCode: '581314', countryCode: 'IN' },
  customerNote: '', items: [{ id: '709991d6-a2be-4916-a4ce-7a79a18387a6', productName: 'Test Saree', productSku: 'AR-TEST-01', variantDescription: 'Size M', quantity: 1, unitPricePaise: 200000, taxPaise: 0, lineTotalPaise: 200000 }],
  events: [{ fromStatus: 'processing', toStatus: 'ready_to_ship', reason: 'Packed', actorName: 'Studio Admin', createdAt: '2026-10-29T10:30:00Z' }],
};
function response(data: unknown) { return new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } }); }
function page() { return response({ data: [order], pagination: { limit: 50, offset: 0, count: 1, hasMore: false } }); }
function detailResponse(value = detail) { return response({ data: value }); }
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('admin order workflow', () => {
  it('loads real order details and ships only with reason, carrier, tracking, and CSRF', async () => {
    const shipped = { ...detail, status: 'shipped', version: 5, shipmentStatus: 'shipped', shipmentProvider: 'Aramya Logistics', trackingNumber: 'TRK-123', trackingUrl: 'https://track.example.test/TRK-123' };
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/admin/orders?')) return page();
      if (url.endsWith(`/admin/orders/${order.id}`) && !init?.method) return detailResponse();
      if (url.endsWith('/auth/csrf')) return response({ csrfToken: 'csrf-order-token' });
      if (url.endsWith(`/admin/orders/${order.id}`) && init?.method === 'PATCH') return detailResponse(shipped);
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminOrdersPage /></MemoryRouter>);
    expect(await screen.findByText('AR-2026-001')).toBeTruthy();
    expect(screen.getByText(/Payment: captured/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'View order' }));
    expect(await screen.findByText('Test Saree')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /mark as paid/i })).toBeNull();
    fireEvent.change(screen.getByLabelText('Carrier'), { target: { value: 'Aramya Logistics' } });
    fireEvent.change(screen.getByLabelText('Tracking number'), { target: { value: 'TRK-123' } });
    fireEvent.change(screen.getByLabelText('Tracking URL (optional)'), { target: { value: 'https://track.example.test/TRK-123' } });
    fireEvent.change(screen.getByLabelText('Reason for change'), { target: { value: 'Carrier collected parcel' } });
    fireEvent.click(screen.getByRole('button', { name: 'Move to shipped' }));
    expect((await screen.findByRole('status')).textContent).toContain('Order moved to shipped.');
    await waitFor(() => expect(fetcher).toHaveBeenCalledWith(expect.stringContaining(`/admin/orders/${order.id}`), expect.objectContaining({ method: 'PATCH' })));
    const update = fetcher.mock.calls.find(([url, init]) => String(url).endsWith(`/admin/orders/${order.id}`) && init?.method === 'PATCH');
    expect(JSON.parse(String(update?.[1]?.body))).toEqual({ expectedVersion: 4, status: 'shipped', reason: 'Carrier collected parcel', shipmentProvider: 'Aramya Logistics', trackingNumber: 'TRK-123', trackingUrl: 'https://track.example.test/TRK-123' });
    expect((update?.[1]?.headers as Record<string, string>)['X-CSRF-Token']).toBe('csrf-order-token');
  });

  it('shows an authorization error instead of a fake empty queue', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'Sign in required' } }), { status: 401 })));
    render(<MemoryRouter><AdminOrdersPage /></MemoryRouter>);
    expect((await screen.findByRole('alert')).textContent).toMatch(/sign in with an administrator/i);
    expect(screen.queryByText('No orders found')).toBeNull();
  });
});
