import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { CartProvider } from '../features/cart/CartContext';
import { CheckoutPage } from './CheckoutPage';

const cartWithLine = { data: {
  lines: [{
    id: '648ba03a-415e-48b3-a247-7c79deab283e',
    variantId: '0e6f52df-72d8-44b2-96be-502aa0184816',
    productId: 'f9f7752d-4489-4b31-9a31-891a7cc13e9e',
    slug: 'heirloom-saree',
    name: 'The Heirloom Saree',
    category: 'sarees',
    categoryName: 'Sarees',
    imageKey: 'image assests/saree1.jpg',
    unitPricePaise: 890000,
    quantity: 1,
  }],
  count: 1,
  subtotalPaise: 890000,
} };

describe('CheckoutPage', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('quotes the server cart and saves a pending order without claiming payment', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(cartWithLine), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'quote-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { subtotalPaise: 890000, totalPaise: 890000, quoteStatus: 'provisional', paymentAvailable: false } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'order-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { id: '27c9d584-0f13-4d75-83a5-2a95a2cdb1ef', orderNumber: 'AR-261029-TESTORDER', status: 'pending_payment', totalPaise: 890000 }, paymentAvailable: false }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { lines: [], count: 0, subtotalPaise: 0 } }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    render(<MemoryRouter><CartProvider><CheckoutPage /></CartProvider></MemoryRouter>);

    await screen.findByRole('button', { name: 'Save order request' });
    fireEvent.change(screen.getByLabelText('Full name'), { target: { value: 'Guest Customer' } });
    fireEvent.change(screen.getByLabelText('Email address'), { target: { value: 'guest@example.test' } });
    fireEvent.change(screen.getByLabelText('Phone number'), { target: { value: '+919876543210' } });
    fireEvent.change(screen.getByLabelText('Address'), { target: { value: '12 Example Road' } });
    fireEvent.change(screen.getByLabelText('City or town'), { target: { value: 'Ankola' } });
    fireEvent.change(screen.getByLabelText('State'), { target: { value: 'Karnataka' } });
    fireEvent.change(screen.getByLabelText('PIN / postal code'), { target: { value: '581314' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save order request' }));

    expect(await screen.findByText(/AR-261029-TESTORDER is saved with status pending payment/)).toBeTruthy();
    expect(screen.getByText(/This order is not confirmed as paid/)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(6);
    expect(fetchMock.mock.calls[2][0]).toContain('/api/v1/checkout/quote');
    expect(fetchMock.mock.calls[4][0]).toContain('/api/v1/checkout/orders');
    const orderHeaders = new Headers(fetchMock.mock.calls[4][1]?.headers);
    expect(orderHeaders.get('X-CSRF-Token')).toBe('order-csrf');
    expect(orderHeaders.get('Idempotency-Key')).toMatch(/^[0-9a-f-]{36}$/i);
    await waitFor(() => expect(fetchMock.mock.calls[5][0]).toContain('/api/v1/cart'));
  });

  it('opens Razorpay only for a final server quote and shows success only after API capture verification', async () => {
    let checkoutOptions: { key: string; amount: number; order_id: string; handler: (response: { razorpay_payment_id: string; razorpay_order_id: string; razorpay_signature: string }) => void | Promise<void> } | undefined;
    class TestRazorpay {
      constructor(options: NonNullable<typeof checkoutOptions>) { checkoutOptions = options; }
      open() { void checkoutOptions?.handler({ razorpay_payment_id: 'pay_test_123456', razorpay_order_id: 'order_test_123456', razorpay_signature: 'a'.repeat(64) }); }
      on() { /* No payment.failed event in this successful provider fixture. */ }
    }
    vi.stubGlobal('Razorpay', TestRazorpay);

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(cartWithLine), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'quote-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { subtotalPaise: 890000, totalPaise: 890000, quoteStatus: 'final', paymentAvailable: true } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'order-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { id: '27c9d584-0f13-4d75-83a5-2a95a2cdb1ef', orderNumber: 'AR-261029-TESTORDER', status: 'pending_payment', totalPaise: 890000 }, paymentAvailable: true }), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { lines: [], count: 0, subtotalPaise: 0 } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'payment-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { orderId: '27c9d584-0f13-4d75-83a5-2a95a2cdb1ef', orderNumber: 'AR-261029-TESTORDER', providerOrderId: 'order_test_123456', keyId: 'rzp_test_public', amountPaise: 890000, currency: 'INR', status: 'pending' } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'verify-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { orderId: '27c9d584-0f13-4d75-83a5-2a95a2cdb1ef', orderNumber: 'AR-261029-TESTORDER', orderStatus: 'paid', paymentStatus: 'captured' } }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    render(<MemoryRouter><CartProvider><CheckoutPage /></CartProvider></MemoryRouter>);

    await screen.findByRole('button', { name: 'Save order request' });
    fireEvent.change(screen.getByLabelText('Full name'), { target: { value: 'Guest Customer' } });
    fireEvent.change(screen.getByLabelText('Email address'), { target: { value: 'guest@example.test' } });
    fireEvent.change(screen.getByLabelText('Phone number'), { target: { value: '+919876543210' } });
    fireEvent.change(screen.getByLabelText('Address'), { target: { value: '12 Example Road' } });
    fireEvent.change(screen.getByLabelText('City or town'), { target: { value: 'Ankola' } });
    fireEvent.change(screen.getByLabelText('State'), { target: { value: 'Karnataka' } });
    fireEvent.change(screen.getByLabelText('PIN / postal code'), { target: { value: '581314' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save order request' }));

    expect(await screen.findByText('PAYMENT CONFIRMED')).toBeTruthy();
    expect(checkoutOptions).toMatchObject({ key: 'rzp_test_public', amount: 890000, order_id: 'order_test_123456' });
    expect(fetchMock).toHaveBeenCalledTimes(10);
    expect(fetchMock.mock.calls[7][0]).toContain('/api/v1/checkout/orders/27c9d584-0f13-4d75-83a5-2a95a2cdb1ef/payment');
    expect(fetchMock.mock.calls[9][0]).toContain('/api/v1/payments/razorpay/verify');
    expect(JSON.parse(String(fetchMock.mock.calls[9][1]?.body))).toMatchObject({
      orderId: '27c9d584-0f13-4d75-83a5-2a95a2cdb1ef',
      razorpayOrderId: 'order_test_123456',
      razorpayPaymentId: 'pay_test_123456',
    });
  });
});
