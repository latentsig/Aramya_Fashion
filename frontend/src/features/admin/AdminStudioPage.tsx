import { useEffect, useState, type FormEvent } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { AdminApiError, fetchAppointments, fetchCustomOrders, updateAppointment, updateCustomOrder, type AppointmentRequest, type CustomOrderRequest } from './studioApi';
import './admin-studio.css';

type Kind = 'appointments' | 'custom-orders';
type Notice = { kind: 'success' | 'error'; text: string } | null;

const appointmentNext: Record<string, string[]> = {
  requested: ['confirmed', 'cancelled'], confirmed: ['completed', 'cancelled'], completed: [], cancelled: [],
};
const customNext: Record<string, string[]> = {
  requested: ['reviewing', 'declined', 'cancelled'], reviewing: ['fitting', 'quoted', 'declined', 'cancelled'],
  fitting: ['quoted', 'declined', 'cancelled'], quoted: ['awaiting_approval', 'declined', 'cancelled'],
  awaiting_approval: ['declined', 'cancelled'], approved: [], in_progress: [], completed: [], declined: [], cancelled: [],
};
const labels: Record<string, string> = {
  requested: 'Requested', confirmed: 'Confirmed', completed: 'Completed', cancelled: 'Cancelled', reviewing: 'Reviewing',
  fitting: 'Fitting', quoted: 'Quoted', awaiting_approval: 'Awaiting customer approval', approved: 'Approved',
  in_progress: 'In progress', declined: 'Declined',
};
const appointmentFilters = ['requested', 'confirmed', 'completed', 'cancelled'];
const customFilters = ['requested', 'reviewing', 'fitting', 'quoted', 'awaiting_approval', 'approved', 'in_progress', 'completed', 'declined', 'cancelled'];

function friendlyError(error: unknown): string {
  if (error instanceof AdminApiError && error.status === 401) return 'Sign in with an administrator account to view studio requests.';
  if (error instanceof AdminApiError && error.status === 403) return 'Your account does not have administrator access.';
  if (error instanceof AdminApiError && error.status === 409) return 'This request has changed. Refresh the queue before trying again.';
  return error instanceof Error ? error.message : 'The request could not be completed. Please retry.';
}

function formatDate(value: string | null): string {
  if (!value) return 'Not specified';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? 'Not specified' : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

function money(paise: number | null): string {
  if (paise === null) return 'Not quoted';
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 2 }).format(paise / 100);
}

function quoteToPaise(value: string): number | null {
  if (!/^\d{1,8}(\.\d{1,2})?$/.test(value)) return null;
  const [rupees, fraction = ''] = value.split('.');
  const paise = Number(rupees) * 100 + Number(fraction.padEnd(2, '0'));
  return Number.isSafeInteger(paise) && paise > 0 ? paise : null;
}

export function AdminStudioPage() {
  const location = useLocation();
  const kind: Kind = location.pathname.includes('custom-orders') ? 'custom-orders' : 'appointments';
  const [rows, setRows] = useState<(AppointmentRequest | CustomOrderRequest)[]>([]);
  const [status, setStatus] = useState('');
  const [searchTerm, setSearchTerm] = useState('');
  const [search, setSearch] = useState('');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState<Notice>(null);
  const [selectedNext, setSelectedNext] = useState<Record<string, string>>({});
  const [quote, setQuote] = useState<Record<string, string>>({});
  const [busyID, setBusyID] = useState('');
  const [retry, setRetry] = useState(0);
  const filters = kind === 'appointments' ? appointmentFilters : customFilters;
  const activeStatus = filters.includes(status) ? status : '';

  useEffect(() => {
    setRows([]);
    setOffset(0);
    setSelectedNext({});
    setStatus('');
  }, [kind]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setFailure('');
    const request = kind === 'appointments'
      ? fetchAppointments({ status: activeStatus, search, offset }, controller.signal)
      : fetchCustomOrders({ status: activeStatus, search, offset }, controller.signal);
    void request.then((page) => {
      setRows((current) => offset === 0 ? page.data : [...current, ...page.data]);
      setHasMore(page.pagination.hasMore);
    }).catch((error: unknown) => {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      setFailure(friendlyError(error));
      setRows([]);
      setHasMore(false);
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [kind, activeStatus, search, offset, retry]);

  function applySearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setOffset(0);
    setSearch(searchTerm.trim());
  }

  async function saveAppointment(item: AppointmentRequest) {
    const target = selectedNext[item.id] ?? appointmentNext[item.status]?.[0];
    if (!target) return;
    setBusyID(item.id); setNotice(null);
    try {
      const updated = await updateAppointment(item.id, item.status, target);
      setRows((current) => current.map((row) => row.id === item.id ? updated : row));
      setSelectedNext((current) => { const next = { ...current }; delete next[item.id]; return next; });
      setNotice({ kind: 'success', text: `Appointment marked ${labels[updated.status].toLowerCase()}.` });
    } catch (error) { setNotice({ kind: 'error', text: friendlyError(error) }); }
    finally { setBusyID(''); }
  }

  async function saveCustom(item: CustomOrderRequest) {
    const target = selectedNext[item.id] ?? customNext[item.status]?.[0];
    if (!target) return;
    const amount = target === 'quoted' ? quoteToPaise(quote[item.id] ?? '') : undefined;
    if (target === 'quoted' && amount === null) {
      setNotice({ kind: 'error', text: 'Enter a valid quotation greater than ₹0.' });
      return;
    }
    setBusyID(item.id); setNotice(null);
    try {
      const updated = await updateCustomOrder(item.id, item.status, target, amount ?? undefined);
      setRows((current) => current.map((row) => row.id === item.id ? updated : row));
      setSelectedNext((current) => { const next = { ...current }; delete next[item.id]; return next; });
      setNotice({ kind: 'success', text: `Custom request moved to ${labels[updated.status].toLowerCase()}.` });
    } catch (error) { setNotice({ kind: 'error', text: friendlyError(error) }); }
    finally { setBusyID(''); }
  }

  return (
    <main className="admin-studio section-pad">
      <div className="admin-studio-heading">
        <div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Studio <em>requests.</em></h1><p>Review appointment enquiries and custom garment requests from the studio queue.</p></div>
        <Link className="admin-studio-back" to="/">Back to storefront ↗</Link>
      </div>
      <nav className="admin-studio-tabs" aria-label="Studio request queues">
        <Link to="/admin/products">Products</Link>
        <Link to="/admin/categories">Categories</Link>
        <Link to="/admin/inventory">Inventory</Link>
        <Link to="/admin/orders">Orders</Link>
        <Link aria-current={kind === 'appointments' ? 'page' : undefined} to="/admin/appointments">Appointments</Link>
        <Link aria-current={kind === 'custom-orders' ? 'page' : undefined} to="/admin/custom-orders">Made to order</Link>
      </nav>
      <form className="admin-studio-filters" onSubmit={applySearch}>
        <label>Status<select aria-label="Filter by status" value={activeStatus} onChange={(event) => { setStatus(event.target.value); setOffset(0); }}><option value="">All statuses</option>{filters.map((item) => <option key={item} value={item}>{labels[item]}</option>)}</select></label>
        <label>Search<input aria-label="Search studio requests" value={searchTerm} onChange={(event) => setSearchTerm(event.target.value)} maxLength={100} placeholder="Name, email or phone" /></label>
        <button className="button button-light" type="submit">Apply filters <span>↗</span></button>
      </form>
      {notice && <p role={notice.kind === 'error' ? 'alert' : 'status'} className={`admin-studio-notice ${notice.kind}`}>{notice.text}</p>}
      {failure && <div className="admin-studio-state" role="alert"><p>{failure}</p>{failure.includes('Sign in') && <Link to="/login">Sign in</Link>}<button type="button" onClick={() => { setOffset(0); setRetry((value) => value + 1); }}>Retry</button></div>}
      {loading && <p className="admin-studio-state" role="status">Loading studio requests…</p>}
      {!loading && !failure && rows.length === 0 && <div className="admin-studio-state"><h2>No requests found</h2><p>Try another status or search, or check again later.</p></div>}
      <div className="admin-studio-list">
        {kind === 'appointments' && (rows as AppointmentRequest[]).map((item) => {
          const options = appointmentNext[item.status] ?? [];
          return <article className="admin-request-card" key={item.id}>
            <div className="admin-request-heading"><div><span className={`admin-status status-${item.status}`}>{labels[item.status]}</span><h2>{item.name}</h2></div><time dateTime={item.createdAt}>Received {formatDate(item.createdAt)}</time></div>
            <div className="admin-request-details"><p><b>Contact</b><span>{item.email || '—'}{item.email && item.phone ? ' · ' : ''}{item.phone || ''}</span></p><p><b>Preferred visit</b><span>{formatDate(item.preferredAt)}</span></p><p><b>Interested in</b><span>{item.interest || 'Not specified'}</span></p>{item.note && <p className="request-note"><b>Note</b><span>{item.note}</span></p>}</div>
            {options.length > 0 && <div className="admin-request-actions"><label>Move to<select aria-label={`Next status for ${item.name}`} value={selectedNext[item.id] ?? options[0]} onChange={(event) => setSelectedNext((current) => ({ ...current, [item.id]: event.target.value }))}>{options.map((option) => <option key={option} value={option}>{labels[option]}</option>)}</select></label><button className="button button-red" disabled={busyID === item.id} onClick={() => void saveAppointment(item)} type="button">{busyID === item.id ? 'Saving…' : 'Save status'}</button></div>}
          </article>;
        })}
        {kind === 'custom-orders' && (rows as CustomOrderRequest[]).map((item) => {
          const options = customNext[item.status] ?? [];
          const target = selectedNext[item.id] ?? options[0] ?? '';
          return <article className="admin-request-card" key={item.id}>
            <div className="admin-request-heading"><div><span className={`admin-status status-${item.status}`}>{labels[item.status]}</span><h2>{item.name}</h2></div><time dateTime={item.createdAt}>Received {formatDate(item.createdAt)}</time></div>
            <div className="admin-request-details"><p><b>Contact</b><span>{item.email || '—'}{item.email && item.phone ? ' · ' : ''}{item.phone || ''}</span></p><p><b>Garment</b><span>{item.garmentType}</span></p><p><b>Occasion</b><span>{item.occasion || 'Not specified'}</span></p><p><b>Current quote</b><span>{money(item.quotationPaise)}</span></p>{item.requirements && <p className="request-note"><b>Requirements</b><span>{item.requirements}</span></p>}{Object.keys(item.measurements).length > 0 && <p className="request-note"><b>Measurements</b><span>{Object.entries(item.measurements).map(([key, value]) => `${key}: ${String(value)}`).join(' · ')}</span></p>}</div>
            {item.status === 'awaiting_approval' && <p className="admin-approval-note">Customer approval is required before work can start. This staff queue cannot approve on the customer’s behalf.</p>}
            {options.length > 0 && <div className="admin-request-actions"><label>Move to<select aria-label={`Next status for ${item.name}`} value={target} onChange={(event) => setSelectedNext((current) => ({ ...current, [item.id]: event.target.value }))}>{options.map((option) => <option key={option} value={option}>{labels[option]}</option>)}</select></label>{target === 'quoted' && <label>Quotation (₹)<input aria-label={`Quotation for ${item.name} in rupees`} inputMode="decimal" type="number" min="0.01" max="999999.99" step="0.01" value={quote[item.id] ?? ''} onChange={(event) => setQuote((current) => ({ ...current, [item.id]: event.target.value }))} /></label>}<button className="button button-red" disabled={busyID === item.id} onClick={() => void saveCustom(item)} type="button">{busyID === item.id ? 'Saving…' : 'Save request'}</button></div>}
          </article>;
        })}
      </div>
      {!loading && !failure && hasMore && <button className="admin-studio-more" type="button" onClick={() => setOffset((value) => value + 50)}>Load 50 more requests</button>}
    </main>
  );
}
