import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminInventoryPage } from './AdminInventoryPage';

const item = {
  variantId: '2b4172b4-2897-4aaa-aa58-9196176f6f03', productId: '709991d6-a2be-4916-a4ce-7a79a18387a6',
  sku: 'AR-SAR-01-M', productName: 'Gulabi Banarasi Saree', productSlug: 'gulabi-banarasi-saree', categoryName: 'Sarees',
  optionName: 'Size', optionValue: 'M', pricePaise: 1499000, inventoryPolicy: 'tracked', active: true,
  onHand: 8, reserved: 2, available: 6, updatedAt: '2026-10-28T10:00:00Z',
};

function page(data: unknown[]) {
  return new Response(JSON.stringify({ data, pagination: { limit: 50, offset: 0, count: data.length, hasMore: false } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('admin inventory page', () => {
  it('shows variant quantities and commits a CSRF-protected stock correction', async () => {
    const saved = { ...item, onHand: 11, available: 9 };
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/api/v1/admin/inventory?')) return page([item]);
      if (url.endsWith('/api/v1/auth/csrf')) return new Response(JSON.stringify({ csrfToken: 'csrf-token' }), { status: 200 });
      if (init?.method === 'PATCH') return new Response(JSON.stringify({ data: saved }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminInventoryPage /></MemoryRouter>);

    expect(await screen.findByRole('heading', { name: 'Gulabi Banarasi Saree' })).toBeTruthy();
    expect(screen.getByText('6')).toBeTruthy();
    fireEvent.change(screen.getByRole('spinbutton', { name: /On-hand quantity/ }), { target: { value: '11' } });
    fireEvent.change(screen.getByRole('textbox', { name: /Adjustment reason/ }), { target: { value: 'Received counted pieces' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save stock' }));

    expect((await screen.findByRole('status')).textContent).toContain('Stock updated for Gulabi Banarasi Saree');
    await waitFor(() => expect(fetcher).toHaveBeenCalledWith(expect.stringContaining(`/admin/inventory/${item.variantId}`), expect.objectContaining({ method: 'PATCH' })));
    const patchCall = fetcher.mock.calls.find(([url, init]) => String(url).includes(item.variantId) && init?.method === 'PATCH');
    expect(JSON.parse(String(patchCall?.[1]?.body))).toEqual({ expectedOnHand: 8, onHand: 11, reason: 'Received counted pieces' });
  });

  it('does not offer stock changes for made-to-order inventory policies', async () => {
    const madeToOrder = { ...item, inventoryPolicy: 'made_to_order', onHand: 0, reserved: 0, available: 0 };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(page([madeToOrder])));
    render(<MemoryRouter><AdminInventoryPage /></MemoryRouter>);
    expect(await screen.findByText(/Stock is not tracked for this made-to-order/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Save stock' })).toBeNull();
  });
});
