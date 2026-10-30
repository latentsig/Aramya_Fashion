import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminOverviewPage } from './AdminOverviewPage';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('admin overview page', () => {
  it('shows server-reported captured sales, queues and recent order status', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {
      activeProducts: 39, totalOrders: 12, pendingPaymentOrders: 4, grossCapturedPaise: 890000, processedRefundsPaise: 10000, netSalesPaise: 880000, capturedPayments: 3,
      requestedAppointments: 2, openCustomOrders: 1,
      recentOrders: [{ orderNumber: 'AR-OVERVIEW-1', status: 'paid', paymentStatus: 'captured', currency: 'INR', totalPaise: 890000, createdAt: '2026-10-30T10:00:00Z' }],
    } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminOverviewPage /></MemoryRouter>);

    expect(await screen.findByText('₹8,800.00')).toBeTruthy();
    expect(screen.getAllByText('₹8,900.00')).toHaveLength(2);
    expect(screen.getByText('3 confirmed captures')).toBeTruthy();
    expect(screen.getByText('AR-OVERVIEW-1')).toBeTruthy();
    expect(screen.getByText('captured')).toBeTruthy();
    expect(fetcher).toHaveBeenCalledWith(expect.stringContaining('/api/v1/admin/overview'), expect.objectContaining({ credentials: 'include' }));
    expect(screen.getByText(/Net sales subtract processed refunds/)).toBeTruthy();
  });

  it('shows access errors and offers retry', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'Administrator access is required' } }), { status: 403 }));
    vi.stubGlobal('fetch', fetcher);
    render(<MemoryRouter><AdminOverviewPage /></MemoryRouter>);
    expect(await screen.findByRole('alert')).toBeTruthy();
    expect(screen.getByText(/does not have administrator access/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy();
  });
});
