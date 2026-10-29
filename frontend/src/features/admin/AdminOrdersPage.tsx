import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { AdminApiError } from './studioApi';
import { fetchAdminOrder, fetchAdminOrders, updateAdminOrder, type AdminOrder, type AdminOrderDetail } from './adminOrdersApi';
import './admin-orders.css';

const orderStatuses: AdminOrder['status'][] = ['pending_payment', 'paid', 'confirmed', 'processing', 'ready_to_ship', 'shipped', 'delivered', 'cancelled', 'refund_pending', 'refunded'];
const nextStatus: Partial<Record<AdminOrder['status'], AdminOrder['status']>> = { paid: 'confirmed', confirmed: 'processing', processing: 'ready_to_ship', ready_to_ship: 'shipped', shipped: 'delivered' };
function title(value: string) { return value.replaceAll('_', ' '); }
function money(paise: number, currency: string) { return new Intl.NumberFormat('en-IN', { style: 'currency', currency, maximumFractionDigits: 2 }).format(paise / 100); }
function date(value: string) { return new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
function errorMessage(error: unknown) {
  if (error instanceof AdminApiError && error.status === 401) return 'Sign in with an administrator account to review orders.';
  if (error instanceof AdminApiError && error.status === 403) return 'Your account does not have administrator access.';
  if (error instanceof AdminApiError && error.status === 409) return `${error.message}. Refresh the order before trying again.`;
  return error instanceof Error ? error.message : 'The order request could not be completed.';
}
function adminTabs() { return <nav className="admin-studio-tabs" aria-label="Admin sections"><Link to="/admin/products">Products</Link><Link to="/admin/categories">Categories</Link><Link to="/admin/inventory">Inventory</Link><Link aria-current="page" to="/admin/orders">Orders</Link><Link to="/admin/appointments">Studio requests</Link></nav>; }

export function AdminOrdersPage() {
  const [orders, setOrders] = useState<AdminOrder[]>([]);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [refresh, setRefresh] = useState(0);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState('');
  const [selected, setSelected] = useState<AdminOrderDetail | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [next, setNext] = useState<AdminOrder['status'] | ''>('');
  const [reason, setReason] = useState('');
  const [carrier, setCarrier] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingUrl, setTrackingUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState('');

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setFailure('');
    void fetchAdminOrders({ search, status, offset }, controller.signal).then((page) => {
      setOrders((current) => offset === 0 ? page.data : [...current, ...page.data]); setHasMore(page.pagination.hasMore);
    }).catch((error: unknown) => { if (!(error instanceof DOMException && error.name === 'AbortError')) { setOrders([]); setHasMore(false); setFailure(errorMessage(error)); } })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [search, status, offset, refresh]);

  async function openOrder(order: AdminOrder) {
    setLoadingDetail(true); setFailure(''); setNotice('');
    try {
      const detail = await fetchAdminOrder(order.id); setSelected(detail);
      setNext(nextStatus[detail.status] ?? (detail.status === 'pending_payment' ? 'cancelled' : ''));
      setReason(''); setCarrier(''); setTrackingNumber(''); setTrackingUrl('');
    } catch (error) { setFailure(errorMessage(error)); }
    finally { setLoadingDetail(false); }
  }
  function searchOrders(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setOffset(0); setSearch(searchText.trim()); }
  async function saveTransition(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected || !next) return;
    setSaving(true); setNotice(''); setFailure('');
    try {
      const updated = await updateAdminOrder(selected.id, { expectedVersion: selected.version, status: next, reason, ...(next === 'shipped' ? { shipmentProvider: carrier, trackingNumber, ...(trackingUrl ? { trackingUrl } : {}) } : {}) });
      setSelected(updated); setNext(nextStatus[updated.status] ?? ''); setReason(''); setRefresh((value) => value + 1); setNotice(`Order moved to ${title(updated.status)}.`);
    } catch (error) { setNotice(errorMessage(error)); }
    finally { setSaving(false); }
  }

  return <main className="admin-studio admin-orders section-pad">
    <div className="admin-studio-heading"><div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Orders, <em>in good hands.</em></h1><p>Review customer orders, payment state, and the next safe fulfillment action.</p></div></div>
    {adminTabs()}
    <form className="admin-studio-filters admin-order-filters" onSubmit={searchOrders}>
      <label>Order search<input aria-label="Search orders" value={searchText} onChange={(event) => setSearchText(event.target.value)} maxLength={100} placeholder="Order number, customer, email or phone" /></label>
      <label>Status<select aria-label="Filter orders by status" value={status} onChange={(event) => { setStatus(event.target.value); setOffset(0); }}><option value="">All statuses</option>{orderStatuses.map((value) => <option key={value} value={value}>{title(value)}</option>)}</select></label>
      <button className="button button-light" type="submit">Search orders <span>↗</span></button>
    </form>
    {failure && <p role="alert" className="admin-studio-notice error">{failure}</p>}{loading && <p role="status" className="admin-studio-state">Loading orders…</p>}
    {!loading && !failure && orders.length === 0 && <div className="admin-studio-state"><h2>No orders found</h2><p>Orders appear here after a customer submits checkout.</p></div>}
    <div className="admin-order-list">{orders.map((order) => <article className="admin-order-row" key={order.id}>
      <div className="admin-order-main"><div className="admin-order-heading"><h2>{order.orderNumber}</h2><span className={`admin-status status-${order.status}`}>{title(order.status)}</span></div><p>{order.customerName || 'Guest customer'} · {order.customerEmail || order.customerPhone}</p><small>{date(order.createdAt)} · Payment: {title(order.paymentStatus || 'not started')} · Refund: {title(order.refundStatus || 'none')} · Shipment: {title(order.shipmentStatus)}</small></div>
      <div className="admin-order-total"><b>{money(order.totalPaise, order.currency)}</b><button className="admin-studio-back" type="button" onClick={() => void openOrder(order)}>View order</button></div>
    </article>)}</div>
    {!loading && hasMore && <button className="admin-studio-more" type="button" onClick={() => setOffset((value) => value + 50)}>Load more orders</button>}
    {loadingDetail && <p role="status" className="admin-orders-loading">Loading order details…</p>}
    {selected && <div className="admin-catalog-overlay" role="presentation"><section className="admin-catalog-panel admin-order-panel" role="dialog" aria-modal="true" aria-labelledby="admin-order-title">
      <div className="admin-catalog-panel-head"><div><p className="eyebrow">ORDER · v{selected.version}</p><h2 id="admin-order-title">{selected.orderNumber}</h2><span className={`admin-status status-${selected.status}`}>{title(selected.status)}</span></div><button type="button" aria-label="Close order details" onClick={() => setSelected(null)}>×</button></div>
      {notice && <p role="status" className="admin-studio-notice success">{notice}</p>}
      <section className="admin-order-block"><h3>Customer</h3><p>{selected.customerName || 'Guest customer'}<br />{selected.customerEmail}<br />{selected.customerPhone}</p><h3>Shipping address</h3><p>{String(selected.shippingAddress.recipientName ?? '')}<br />{String(selected.shippingAddress.line1 ?? '')}{selected.shippingAddress.line2 ? `, ${String(selected.shippingAddress.line2)}` : ''}<br />{String(selected.shippingAddress.city ?? '')}, {String(selected.shippingAddress.region ?? '')} {String(selected.shippingAddress.postalCode ?? '')}<br />{String(selected.shippingAddress.countryCode ?? '')}</p>{selected.customerNote && <><h3>Customer note</h3><p>{selected.customerNote}</p></>}</section>
      <section className="admin-order-block"><h3>Payment, refund and shipment</h3><p>Payment: <strong>{title(selected.paymentStatus || 'not started')}</strong><br />Refund: <strong>{title(selected.refundStatus || 'none')}</strong> · Processed {money(selected.refundedPaise, selected.currency)}<br />Shipment: <strong>{title(selected.shipmentStatus)}</strong>{selected.shipmentProvider && <> · {selected.shipmentProvider}</>}{selected.trackingNumber && <><br />Tracking: {selected.trackingNumber}</>}{selected.trackingUrl && <><br /><a href={selected.trackingUrl} target="_blank" rel="noreferrer">Open carrier tracking ↗</a></>}</p><p className="admin-order-no-payment">Payment and refund status are read-only here. Staff cannot mark an order paid or issue refunds from fulfillment.</p></section>
      <section className="admin-order-block"><h3>Items</h3>{selected.items.length ? selected.items.map((item) => <div className="admin-order-item" key={item.id}><span><strong>{item.productName}</strong><small>{item.productSku} · {item.variantDescription} · Qty {item.quantity}</small></span><b>{money(item.lineTotalPaise, selected.currency)}</b></div>) : <p>Item details are not available.</p>}<div className="admin-order-money"><span>Subtotal</span><b>{money(selected.subtotalPaise, selected.currency)}</b><span>Tax</span><b>{money(selected.taxPaise, selected.currency)}</b><span>Delivery</span><b>{money(selected.shippingPaise, selected.currency)}</b><span>Total</span><b>{money(selected.totalPaise, selected.currency)}</b></div></section>
      <section className="admin-order-block"><h3>Status history</h3><ol className="admin-order-events">{selected.events.map((event, index) => <li key={`${event.toStatus}-${event.createdAt}-${index}`}><strong>{title(event.toStatus)}</strong><small>{event.fromStatus ? `${title(event.fromStatus)} → ` : ''}{date(event.createdAt)}{event.actorName ? ` · ${event.actorName}` : ''}</small>{event.reason && <p>{event.reason}</p>}</li>)}</ol></section>
      {next && <form className="admin-order-transition" onSubmit={(event) => void saveTransition(event)}><h3>Update order</h3><label>Next state<select required value={next} onChange={(event) => setNext(event.target.value as AdminOrder['status'])}><option value="">Choose next action</option>{(selected.status === 'pending_payment' ? ['cancelled'] : nextStatus[selected.status] ? [nextStatus[selected.status]!] : []).map((value) => <option key={value} value={value}>{title(value)}</option>)}</select></label>
        {next === 'shipped' && <><label>Carrier<input required maxLength={80} value={carrier} onChange={(event) => setCarrier(event.target.value)} /></label><label>Tracking number<input required maxLength={100} value={trackingNumber} onChange={(event) => setTrackingNumber(event.target.value)} /></label><label>Tracking URL (optional)<input type="url" pattern="https://.*" maxLength={500} value={trackingUrl} onChange={(event) => setTrackingUrl(event.target.value)} placeholder="https://" /></label></>}
        <label>Reason for change<textarea required minLength={3} maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} /></label><p>Payment and refund states cannot be set from order fulfillment.</p><button className="button button-red" type="submit" disabled={saving || !next}>{saving ? 'Saving…' : `Move to ${title(next)}`}</button>
      </form>}
    </section></div>}
  </main>;
}
