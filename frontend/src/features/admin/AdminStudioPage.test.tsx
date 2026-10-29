import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminStudioPage } from './AdminStudioPage';

const appointment = {
  id: '2b4172b4-2897-4aaa-aa58-9196176f6f03', name: 'Nila Rao', email: 'nila@example.com', phone: '+919876543210',
  preferredAt: null, interest: 'Bridal fitting', note: 'December ceremony', status: 'requested',
  createdAt: '2026-10-28T10:00:00Z', updatedAt: '2026-10-28T10:00:00Z',
};
const customRequest = {
  id: '709991d6-a2be-4916-a4ce-7a79a18387a6', name: 'Mira Shah', email: 'mira@example.com', phone: '+919876543210',
  garmentType: 'Saree blouse', occasion: 'Wedding', requirements: 'Hand-finished neckline', measurements: {},
  quotationPaise: null, currency: 'INR', status: 'reviewing', createdAt: '2026-10-28T10:00:00Z', updatedAt: '2026-10-28T10:00:00Z',
};

function page(data: unknown[]) {
  return new Response(JSON.stringify({ data, pagination: { limit: 50, offset: 0, count: data.length, hasMore: false } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('admin studio queues', () => {
  it('loads persisted appointment data and offers only valid next statuses', async () => {
    const fetcher = vi.fn().mockResolvedValue(page([appointment]));
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter initialEntries={['/admin/appointments']}><AdminStudioPage /></MemoryRouter>);

    expect(await screen.findByRole('heading', { name: 'Nila Rao' })).toBeTruthy();
    expect(screen.getByText('December ceremony')).toBeTruthy();
    const nextStatus = screen.getByRole('combobox', { name: 'Next status for Nila Rao' });
    expect(Array.from(nextStatus.querySelectorAll('option')).map((option) => option.value)).toEqual(['confirmed', 'cancelled']);
  });

  it('requires authentication and explains missing administrator access', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'Administrator access is required' } }), { status: 403 })));
    render(<MemoryRouter initialEntries={['/admin/appointments']}><AdminStudioPage /></MemoryRouter>);
    expect((await screen.findByRole('alert')).textContent).toContain('Your account does not have administrator access.');
  });

  it('issues a staff quotation in paise through the CSRF-protected API', async () => {
    const quoted = { ...customRequest, status: 'quoted', quotationPaise: 890050 };
    const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/admin/custom-order-requests?')) return page([customRequest]);
      if (url.endsWith('/api/v1/auth/csrf')) return new Response(JSON.stringify({ csrfToken: 'csrf-token' }), { status: 200 });
      if (init?.method === 'PATCH') return new Response(JSON.stringify({ data: quoted }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter initialEntries={['/admin/custom-orders']}><AdminStudioPage /></MemoryRouter>);

    expect(await screen.findByRole('heading', { name: 'Mira Shah' })).toBeTruthy();
    fireEvent.change(screen.getByRole('combobox', { name: 'Next status for Mira Shah' }), { target: { value: 'quoted' } });
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Quotation for Mira Shah in rupees' }), { target: { value: '8900.50' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save request' }));

    expect((await screen.findByRole('status')).textContent).toContain('Custom request moved to quoted.');
    await waitFor(() => expect(fetcher).toHaveBeenCalledWith(expect.stringContaining('/admin/custom-order-requests/709991d6-a2be-4916-a4ce-7a79a18387a6'), expect.objectContaining({ method: 'PATCH' })));
    const patchCall = fetcher.mock.calls.find(([input, init]) => String(input).includes('/admin/custom-order-requests/709991d6') && init?.method === 'PATCH');
    expect(JSON.parse(String(patchCall?.[1]?.body))).toEqual({ expectedStatus: 'reviewing', status: 'quoted', quotationPaise: 890050 });
    expect(new Headers(patchCall?.[1]?.headers).get('X-CSRF-Token')).toBe('csrf-token');
  });
});
