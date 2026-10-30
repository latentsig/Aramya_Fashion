import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { formatPrice } from '../features/products/catalog';
import { useCart } from '../features/cart/CartContext';

type Quote = { subtotalPaise: number; totalPaise: number; quoteStatus: string; paymentAvailable: boolean };
type Order = { id: string; orderNumber: string; status: string; totalPaise: number };
type PaymentSession = { orderId: string; orderNumber: string; providerOrderId: string; keyId: string; amountPaise: number; currency: string };
type PaymentVerification = { orderId: string; orderNumber: string; orderStatus: string; paymentStatus: string };
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

type RazorpayCallback = { razorpay_payment_id: string; razorpay_order_id: string; razorpay_signature: string };
type RazorpayOptions = {
  key: string; amount: number; currency: string; order_id: string; name: string; description: string;
  theme: { color: string };
  handler: (result: RazorpayCallback) => void;
  modal: { ondismiss: () => void };
};
declare global { interface Window { Razorpay?: new (options: RazorpayOptions) => { open: () => void; on: (event: string, callback: (error: { description?: string }) => void) => void } } }

async function openRazorpay(session: PaymentSession, onResult: (result: RazorpayCallback) => void, onDismiss: () => void, onFailure: (message: string) => void) {
  if (!window.Razorpay) {
    await new Promise<void>((resolve, reject) => {
      const script = document.createElement('script');
      script.src = 'https://checkout.razorpay.com/v1/checkout.js';
      script.async = true;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error('Razorpay checkout could not be loaded. Please try again while online.'));
      document.head.appendChild(script);
    });
  }
  if (!window.Razorpay) throw new Error('Razorpay checkout is unavailable in this browser.');
  const checkout = new window.Razorpay({
    key: session.keyId,
    amount: session.amountPaise,
    currency: session.currency,
    order_id: session.providerOrderId,
    name: 'Aramya Fashion House',
    description: `Order ${session.orderNumber}`,
    theme: { color: '#8c1721' },
    handler: onResult,
    modal: { ondismiss: onDismiss },
  });
  checkout.on('payment.failed', (failure) => onFailure(failure.description ?? 'Payment was not completed. You can retry from your order.'));
  checkout.open();
}

async function postJSON<T>(path: string, body: unknown, extraHeaders: Record<string, string> = {}): Promise<T> {
  const csrfResponse = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const csrfPayload = await csrfResponse.json().catch(() => null) as { csrfToken?: string } | null;
  if (!csrfResponse.ok || !csrfPayload?.csrfToken) throw new Error('Could not prepare a secure checkout request. Please retry.');
  const response = await fetch(`${apiBase}/api/v1${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrfPayload.csrfToken, ...extraHeaders },
    body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => null) as { data?: T; error?: { message?: string }; notice?: string } | null;
  if (!response.ok || !payload?.data) throw new Error(payload?.error?.message ?? 'Checkout could not be completed. Please retry.');
  return payload.data;
}

export function CheckoutPage() {
  const { lines, subtotal, loading, error: cartError, refresh } = useCart();
  const [submitting, setSubmitting] = useState(false);
  const [paymentStarting, setPaymentStarting] = useState(false);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [order, setOrder] = useState<Order | null>(null);
  const idempotencyKey = useRef(crypto.randomUUID());
  const paymentIdempotencyKey = useRef(crypto.randomUUID());

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCheckoutError(null);
    setSubmitting(true);
    const form = new FormData(event.currentTarget);
    const details = {
      email: String(form.get('email') ?? ''),
      phone: String(form.get('phone') ?? ''),
      customerNote: String(form.get('notes') ?? ''),
      address: {
        recipientName: String(form.get('name') ?? ''),
        phone: String(form.get('phone') ?? ''),
        line1: String(form.get('address') ?? ''),
        line2: String(form.get('address2') ?? ''),
        city: String(form.get('city') ?? ''),
        region: String(form.get('state') ?? ''),
        postalCode: String(form.get('postal') ?? ''),
        countryCode: String(form.get('country') ?? 'IN'),
      },
    };
    try {
      const nextQuote = await postJSON<Quote>('/checkout/quote', {});
      setQuote(nextQuote);
      const nextOrder = await postJSON<Order>('/checkout/orders', details, { 'Idempotency-Key': idempotencyKey.current });
      setOrder(nextOrder);
      await refresh();
      if (nextQuote.paymentAvailable) {
        setPaymentStarting(true);
        const session = await postJSON<PaymentSession>(`/checkout/orders/${nextOrder.id}/payment`, {}, { 'Idempotency-Key': paymentIdempotencyKey.current });
        await openRazorpay(session, async (callback) => {
          try {
            const result = await postJSON<PaymentVerification>('/payments/razorpay/verify', {
              orderId: nextOrder.id,
              razorpayOrderId: callback.razorpay_order_id,
              razorpayPaymentId: callback.razorpay_payment_id,
              razorpaySignature: callback.razorpay_signature,
            });
            if (result.orderStatus === 'paid' && result.paymentStatus === 'captured') {
              setOrder((current) => current ? { ...current, status: 'paid' } : current);
              setCheckoutError(null);
            } else {
              setCheckoutError('Payment was verified but is not captured yet. The order is still awaiting confirmation.');
            }
          } catch (cause) {
            setCheckoutError(cause instanceof Error ? cause.message : 'Payment confirmation is pending. Please check your order status before retrying.');
          }
        }, () => setCheckoutError('Checkout was closed. Your order remains pending payment.'), setCheckoutError);
        setPaymentStarting(false);
      }
    } catch (cause) {
      setCheckoutError(cause instanceof Error ? cause.message : 'Checkout could not be completed. Please retry.');
    } finally {
      setPaymentStarting(false);
      setSubmitting(false);
    }
  }

  if (loading) return <main className="section-pad"><p role="status">Loading your bag…</p></main>;
  if (!lines.length && !order) return <main className="section-pad"><h1>Your bag is empty.</h1>{cartError && <p role="alert">{cartError}</p>}<Link to="/collections">Explore collections ↗</Link></main>;
  if (order) return <main className="section-pad checkout-page"><div className="checkout-title"><p className="eyebrow">{order.status === 'paid' ? 'PAYMENT CONFIRMED' : 'ORDER REQUEST SAVED'}</p><h1>Thank you for <em>choosing Aramya.</em></h1><p>Order {order.orderNumber} is saved with status {order.status.replaceAll('_', ' ')}.</p></div><section className="checkout-summary"><p>Status: <strong>{order.status.replaceAll('_', ' ')}</strong></p><p>Order total: <strong>{formatPrice(order.totalPaise / 100)}</strong></p>{paymentStarting && <p role="status">Preparing secure Razorpay test checkout…</p>}{checkoutError && <p role="alert" className="checkout-note">{checkoutError}</p>}{order.status !== 'paid' && <p className="checkout-note">This order is not confirmed as paid. Check your order status before trying another payment.</p>}<Link className="button button-red" to="/collections">Continue browsing ↗</Link></section></main>;

  return (
    <main className="section-pad checkout-page">
      <div className="checkout-title"><p className="eyebrow">MADE TO ORDER, MADE PERSONAL</p><h1>Let’s make it <em>yours.</em></h1><p>The studio will confirm fit and availability. Payment opens only when this server has a confirmed payable quote.</p></div>
      <div className="checkout-layout">
        <form className="checkout-form" onSubmit={submit}>
          <p className="eyebrow">01 · YOUR DETAILS</p>
          <div className="checkout-fields">
            <label className="span-two">Full name<input name="name" autoComplete="name" required maxLength={120} /></label>
            <label>Email address<input type="email" name="email" autoComplete="email" required maxLength={254} /></label>
            <label>Phone number<input type="tel" name="phone" autoComplete="tel" required minLength={7} maxLength={32} /></label>
            <label className="span-two">Address<input name="address" autoComplete="street-address" required minLength={2} maxLength={200} /></label>
            <label>Address line 2<input name="address2" autoComplete="address-line2" maxLength={200} /></label>
            <label>City or town<input name="city" autoComplete="address-level2" required maxLength={120} /></label>
            <label>State<input name="state" autoComplete="address-level1" required maxLength={120} /></label>
            <label>PIN / postal code<input name="postal" autoComplete="postal-code" required minLength={3} maxLength={24} /></label>
            <label>Country<select name="country"><option value="IN">India</option><option value="US">United States</option></select></label>
            <label className="span-two">Notes for the studio<textarea name="notes" rows={3} maxLength={1000} /></label>
          </div>
          {checkoutError && <p role="alert" className="checkout-note">{checkoutError}</p>}
          <p className="checkout-note">Submitting first saves a pending order. When this deployment has approved test credentials and confirmed payable pricing, Razorpay test checkout opens next; otherwise no payment is taken.</p>
          <button type="submit" className="button button-red checkout-submit" disabled={submitting}>{submitting ? 'Saving your order…' : 'Save order request'}</button>
        </form>
        <aside className="checkout-summary">
          <p className="eyebrow">YOUR SELECTION</p><h2>Order summary</h2>
          {lines.map(({ product, quantity, variantId }) => <p className="checkout-row" key={variantId}><span>{quantity} × {product.name}</span><strong>{formatPrice(product.priceRupees * quantity)}</strong></p>)}
          <p className="summary-total"><span>Cart subtotal</span><strong>{formatPrice(subtotal)}</strong></p>
          {quote && <div role="status"><p className="checkout-row"><span>{quote.quoteStatus === 'final' ? 'Final payable quote' : 'Provisional quote'}</span><strong>{formatPrice(quote.totalPaise / 100)}</strong></p><small>{quote.quoteStatus === 'final' ? 'This amount is the confirmed total for the current checkout.' : 'Shipping and tax are not configured; this estimate is not a payable total.'}</small></div>}
          <Link className="text-link" to="/cart">Edit bag ↗</Link>
        </aside>
      </div>
    </main>
  );
}
