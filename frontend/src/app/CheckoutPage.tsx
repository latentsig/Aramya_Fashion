import { useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { formatPrice } from '../features/products/catalog';
import { useCart } from '../features/cart/CartContext';

type Quote = { subtotalPaise: number; totalPaise: number; quoteStatus: string; paymentAvailable: boolean };
type Order = { id: string; orderNumber: string; status: string; totalPaise: number };
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

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
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [order, setOrder] = useState<Order | null>(null);
  const idempotencyKey = useRef(crypto.randomUUID());

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
    } catch (cause) {
      setCheckoutError(cause instanceof Error ? cause.message : 'Checkout could not be completed. Please retry.');
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) return <main className="section-pad"><p role="status">Loading your bag…</p></main>;
  if (!lines.length && !order) return <main className="section-pad"><h1>Your bag is empty.</h1>{cartError && <p role="alert">{cartError}</p>}<Link to="/collections">Explore collections ↗</Link></main>;
  if (order) return <main className="section-pad checkout-page"><div className="checkout-title"><p className="eyebrow">ORDER REQUEST SAVED</p><h1>Thank you for <em>choosing Aramya.</em></h1><p>Order {order.orderNumber} is saved as pending payment. Online payment is unavailable until the Razorpay test account is configured.</p></div><section className="checkout-summary"><p>Status: <strong>{order.status.replaceAll('_', ' ')}</strong></p><p>Provisional total: <strong>{formatPrice(order.totalPaise / 100)}</strong></p><p className="checkout-note">This is not a completed payment. The studio must confirm delivery and applicable charges before payment can be enabled.</p><Link className="button button-red" to="/collections">Continue browsing ↗</Link></section></main>;

  return (
    <main className="section-pad checkout-page">
      <div className="checkout-title"><p className="eyebrow">MADE TO ORDER, MADE PERSONAL</p><h1>Let’s make it <em>yours.</em></h1><p>Share your details so our Ankola studio can confirm fit, availability and delivery with you.</p></div>
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
          <p className="checkout-note">Submitting saves a pending order request. It does not charge you. Payment stays disabled until test credentials and delivery/tax rules are configured.</p>
          <button type="submit" className="button button-red checkout-submit" disabled={submitting}>{submitting ? 'Saving your order…' : 'Save order request'}</button>
        </form>
        <aside className="checkout-summary">
          <p className="eyebrow">YOUR SELECTION</p><h2>Order summary</h2>
          {lines.map(({ product, quantity, variantId }) => <p className="checkout-row" key={variantId}><span>{quantity} × {product.name}</span><strong>{formatPrice(product.priceRupees * quantity)}</strong></p>)}
          <p className="summary-total"><span>Cart subtotal</span><strong>{formatPrice(subtotal)}</strong></p>
          {quote && <div role="status"><p className="checkout-row"><span>Provisional quote</span><strong>{formatPrice(quote.totalPaise / 100)}</strong></p><small>Shipping and tax are not configured; this estimate is not a payable total.</small></div>}
          <Link className="text-link" to="/cart">Edit bag ↗</Link>
        </aside>
      </div>
    </main>
  );
}
