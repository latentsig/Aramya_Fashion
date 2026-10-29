import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { StudioRequestDialog } from './StudioRequestDialog';

const savedRequest = { data: { id: 'd2a8b7d1-7ab6-4579-bcf3-d3096841e76e', status: 'requested', createdAt: '2026-10-29T10:00:00Z' } };

function mockStudioAPI() {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (url.endsWith('/api/v1/auth/csrf')) return new Response(JSON.stringify({ csrfToken: 'csrf-test-token' }), { status: 200 });
    if (url.endsWith('/api/v1/appointments') || url.endsWith('/api/v1/custom-order-requests')) return new Response(JSON.stringify(savedRequest), { status: 201 });
    return new Response(JSON.stringify({ error: { message: `Unexpected request ${url} ${init?.method}` } }), { status: 500 });
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('studio request dialog', () => {
  afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

  it('saves an appointment request through the CSRF-protected API', async () => {
    const fetchMock = mockStudioAPI();
    render(<><a href="#appointment">Book a studio appointment</a><StudioRequestDialog /></>);
    fireEvent.click(screen.getByRole('link', { name: 'Book a studio appointment' }));

    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Your name'), { target: { value: 'Aramya Test Guest' } });
    fireEvent.change(within(dialog).getByLabelText(/email address/i), { target: { value: 'studio-test@example.com' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /send appointment request/i }));

    await waitFor(() => expect(fetchMock.mock.calls.some(([input, init]) => String(input).endsWith('/api/v1/appointments') && init?.method === 'POST')).toBe(true));
    expect(await within(dialog).findByText(/this is a request, not a confirmed appointment/i)).toBeTruthy();
    const post = fetchMock.mock.calls.find(([input, init]) => String(input).endsWith('/api/v1/appointments') && init?.method === 'POST');
    expect(post).toBeTruthy();
    expect(new Headers(post?.[1]?.headers).get('X-CSRF-Token')).toBe('csrf-test-token');
    expect(new Headers(post?.[1]?.headers).get('Idempotency-Key')).toMatch(/^[0-9a-f-]{36}$/i);
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({ name: 'Aramya Test Guest', email: 'studio-test@example.com', interest: 'Bridal or wedding wear' });
  });

  it('opens the custom inquiry form and keeps it as a quotation request', async () => {
    const fetchMock = mockStudioAPI();
    render(<><a href="#custom-order">Start a custom order</a><StudioRequestDialog /></>);
    fireEvent.click(screen.getByRole('link', { name: 'Start a custom order' }));

    const dialog = await screen.findByRole('dialog');
    expect(await within(dialog).findByRole('heading', { name: 'Tell us what you have in mind' })).toBeTruthy();
    fireEvent.change(within(dialog).getByLabelText('Your name'), { target: { value: 'Aramya Test Guest' } });
    fireEvent.change(within(dialog).getByLabelText(/phone number/i), { target: { value: '+91 98765 43210' } });
    fireEvent.change(within(dialog).getByLabelText('Garment type'), { target: { value: 'Lehenga' } });
    fireEvent.change(within(dialog).getByLabelText(/occasion/i), { target: { value: 'Wedding' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /send custom order inquiry/i }));

    await waitFor(() => expect(fetchMock.mock.calls.some(([input, init]) => String(input).endsWith('/api/v1/custom-order-requests') && init?.method === 'POST')).toBe(true));
    expect(await within(dialog).findByText(/discuss fit, timing and a quotation before any order or payment is agreed/i)).toBeTruthy();
    const post = fetchMock.mock.calls.find(([input, init]) => String(input).endsWith('/api/v1/custom-order-requests') && init?.method === 'POST');
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({ name: 'Aramya Test Guest', phone: '+91 98765 43210', garmentType: 'Lehenga', occasion: 'Wedding' });
  });
});
