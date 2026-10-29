import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, Navigate, useParams } from 'react-router-dom';
import { formatPrice } from '../features/products/catalog';
import { fetchOrder, fetchOrders } from '../features/orders/api';
import { useAuth } from '../features/auth/AuthContext';

const dateFormatter = new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium', timeStyle: 'short' });

function orderDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : dateFormatter.format(date);
}

function orderStatus(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function AccountOrdersPage() {
  const { user, loading: authLoading } = useAuth();
  const [offset, setOffset] = useState(0);
  const query = useQuery({
    queryKey: ['account-orders', user?.id, offset],
    queryFn: ({ signal }) => fetchOrders(offset, signal),
    enabled: Boolean(user),
    retry: false,
  });

  if (authLoading) return <main className="section-pad"><p role="status">Loading your account…</p></main>;
  if (!user) return <Navigate to="/login" replace />;

  return <main className="orders-page section-pad">
    <Link className="orders-back" to="/account">← Your account</Link>
    <p className="eyebrow">YOUR ARAMYA ACCOUNT</p>
    <h1>Your <em>orders.</em></h1>
    <p className="orders-intro">A record of the pieces and moments you’ve chosen with Aramya.</p>
    {query.isPending && <p className="orders-state" role="status">Loading your orders…</p>}
    {query.isError && <section className="orders-state" role="alert"><p>{query.error.message}</p><button type="button" className="text-link" onClick={() => void query.refetch()}>Try again ↗</button></section>}
    {query.data && query.data.data.length === 0 && <section className="orders-empty"><h2>No orders yet.</h2><p>Your next occasion piece may be waiting in the collection.</p><Link className="button button-red" to="/collections">Explore collections <span aria-hidden="true">↗</span></Link></section>}
    {query.data && query.data.data.length > 0 && <>
      <div className="order-list">{query.data.data.map((order) => <article className="order-card" key={order.id}>
        <div className="order-card-main"><p className="order-kicker">ORDER {order.orderNumber}</p><h2>{formatPrice(order.totalPaise / 100)}</h2><p className="order-date">Placed {orderDate(order.createdAt)}</p></div>
        <div className="order-card-aside"><span className={`order-status order-status-${order.status}`}>{orderStatus(order.status)}</span><Link className="text-link" to={`/account/orders/${order.id}`}>View details <span aria-hidden="true">↗</span></Link></div>
      </article>)}</div>
      <nav className="order-pagination" aria-label="Order history pages">
        <button type="button" className="text-link" disabled={offset === 0 || query.isFetching} onClick={() => setOffset(Math.max(0, offset - 20))}>← Previous</button>
        <span>Showing {offset + 1}–{offset + query.data.pagination.count}</span>
        <button type="button" className="text-link" disabled={!query.data.pagination.hasMore || query.isFetching} onClick={() => setOffset(offset + 20)}>Next →</button>
      </nav>
    </>}
  </main>;
}

export function OrderDetailPage() {
  const { user, loading: authLoading } = useAuth();
  const { id = '' } = useParams();
  const query = useQuery({
    queryKey: ['account-order', user?.id, id],
    queryFn: ({ signal }) => fetchOrder(id, signal),
    enabled: Boolean(user && id),
    retry: false,
  });

  if (authLoading) return <main className="section-pad"><p role="status">Loading your account…</p></main>;
  if (!user) return <Navigate to="/login" replace />;

  return <main className="orders-page section-pad">
    <Link className="orders-back" to="/account/orders">← Order history</Link>
    {query.isPending && <p className="orders-state" role="status">Loading order details…</p>}
    {query.isError && <section className="orders-state" role="alert"><h1>We couldn’t load this order.</h1><p>{query.error.message}</p><Link className="text-link" to="/account/orders">Return to order history ↗</Link></section>}
    {query.data && <>
      <p className="eyebrow">ORDER {query.data.orderNumber}</p>
      <h1>Thank you for <em>choosing Aramya.</em></h1>
      <div className="order-detail-heading"><span className={`order-status order-status-${query.data.status}`}>{orderStatus(query.data.status)}</span><span>Placed {orderDate(query.data.createdAt)}</span></div>
      {query.data.status === 'pending_payment' && <p className="orders-payment-note">This order is saved as pending payment. Online payment is not enabled here; the studio will confirm the next steps with you.</p>}
      <div className="order-detail-grid">
        <section className="order-detail-panel"><h2>Your pieces</h2><div className="order-item-list">{query.data.items.map((item) => <article className="order-item" key={item.id}>
          <div><h3>{item.productName}</h3><p>{item.productSku} · {item.variantDescription}</p><span>Quantity {item.quantity}</span></div>
          <strong>{formatPrice(item.lineTotalPaise / 100)}</strong>
        </article>)}</div>
          <div className="order-totals"><p><span>Subtotal</span><strong>{formatPrice(query.data.subtotalPaise / 100)}</strong></p>
            {query.data.discountPaise > 0 && <p><span>Discount</span><strong>−{formatPrice(query.data.discountPaise / 100)}</strong></p>}
            {query.data.taxPaise > 0 && <p><span>Tax</span><strong>{formatPrice(query.data.taxPaise / 100)}</strong></p>}
            {query.data.shippingPaise > 0 && <p><span>Delivery</span><strong>{formatPrice(query.data.shippingPaise / 100)}</strong></p>}
            <p className="order-total"><span>Total</span><strong>{formatPrice(query.data.totalPaise / 100)}</strong></p>
          </div>
        </section>
        <section className="order-detail-panel order-address"><h2>Delivery address</h2><address>
          <strong>{query.data.shippingAddress.recipientName}</strong><br />
          {query.data.shippingAddress.line1}{query.data.shippingAddress.line2 && <><br />{query.data.shippingAddress.line2}</>}<br />
          {query.data.shippingAddress.city}, {query.data.shippingAddress.region} {query.data.shippingAddress.postalCode}<br />
          {query.data.shippingAddress.countryCode}<br />
          {query.data.shippingAddress.phone}
        </address>
          {query.data.customerNote && <><h3>Your note</h3><p>{query.data.customerNote}</p></>}
        </section>
      </div>
      <Link className="text-link order-detail-back" to="/account/orders">← All orders</Link>
    </>}
  </main>;
}
