import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { AdminApiError } from './studioApi';
import { fetchAdminOverview, type AdminOverview } from './adminOverviewApi';
import './admin-overview.css';

function money(paise: number, currency = 'INR') {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency, maximumFractionDigits: 2 }).format(paise / 100);
}
function friendly(error: unknown) {
  if (error instanceof AdminApiError && error.status === 401) return 'Sign in with an administrator account to view the business overview.';
  if (error instanceof AdminApiError && error.status === 403) return 'Your account does not have administrator access.';
  return error instanceof Error ? error.message : 'The business overview could not be loaded.';
}

export function AdminOverviewPage() {
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState('');
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setFailure('');
    void fetchAdminOverview(controller.signal).then(setOverview).catch((error: unknown) => {
      if (!(error instanceof DOMException && error.name === 'AbortError')) { setFailure(friendly(error)); setOverview(null); }
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [retry]);

  return <main className="admin-studio admin-overview section-pad">
    <div className="admin-studio-heading">
      <div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Business <em>overview.</em></h1><p>Review verified sales, orders and the work waiting for your team.</p></div>
      <Link className="admin-studio-back" to="/">Back to storefront ↗</Link>
    </div>
    <nav className="admin-studio-tabs" aria-label="Admin sections">
      <Link aria-current="page" to="/admin">Overview</Link><Link to="/admin/products">Products</Link><Link to="/admin/categories">Categories</Link>
      <Link to="/admin/inventory">Inventory</Link><Link to="/admin/orders">Orders</Link><Link to="/admin/appointments">Studio requests</Link>
    </nav>
    {loading && <p role="status" className="admin-overview-state">Loading business overview…</p>}
    {failure && <div role="alert" className="admin-overview-state"><p>{failure}</p>{failure.includes('Sign in') && <Link to="/login">Sign in</Link>}<button type="button" onClick={() => setRetry((value) => value + 1)}>Retry</button></div>}
    {overview && <>
      <p className="admin-overview-sales-note">Gross captures include verified captures later refunded. Net sales subtract processed refunds; pending and unverified payments are excluded.</p>
      <section className="admin-overview-cards" aria-label="Business metrics">
        <article><p>Net sales</p><strong>{money(overview.netSalesPaise)}</strong><small>After {money(overview.processedRefundsPaise)} in processed refunds</small></article>
        <article><p>Gross captured</p><strong>{money(overview.grossCapturedPaise)}</strong><small>{overview.capturedPayments} confirmed captures</small></article>
        <article><p>Orders</p><strong>{overview.totalOrders}</strong><small>{overview.pendingPaymentOrders} pending payment</small></article>
        <article><p>Active products</p><strong>{overview.activeProducts}</strong><small><Link to="/admin/products">Manage catalog ↗</Link></small></article>
        <article><p>Studio queue</p><strong>{overview.requestedAppointments + overview.openCustomOrders}</strong><small>{overview.requestedAppointments} appointments · {overview.openCustomOrders} custom requests</small></article>
      </section>
      <section className="admin-overview-recent" aria-labelledby="recent-orders-title">
        <div className="admin-overview-recent-heading"><div><p className="eyebrow">LATEST ACTIVITY</p><h2 id="recent-orders-title">Recent orders</h2></div><Link to="/admin/orders">View all orders ↗</Link></div>
        {overview.recentOrders.length === 0 ? <p className="admin-overview-empty">No orders have been recorded yet.</p> : <div className="admin-overview-table-wrap"><table>
          <thead><tr><th>Order</th><th>Created</th><th>Order status</th><th>Payment</th><th>Total</th></tr></thead>
          <tbody>{overview.recentOrders.map((order) => <tr key={order.orderNumber}>
            <td><Link to="/admin/orders">{order.orderNumber}</Link></td>
            <td>{new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium' }).format(new Date(order.createdAt))}</td>
            <td>{order.status.replaceAll('_', ' ')}</td><td>{order.paymentStatus || 'not started'}</td><td>{money(order.totalPaise, order.currency)}</td>
          </tr>)}</tbody>
        </table></div>}
      </section>
    </>}
  </main>;
}
