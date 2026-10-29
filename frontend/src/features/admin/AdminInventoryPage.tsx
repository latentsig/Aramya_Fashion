import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { AdminApiError } from './studioApi';
import { fetchInventory, setInventoryOnHand, type InventoryItem } from './inventoryApi';
import './admin-inventory.css';

function errorText(error: unknown): string {
  if (error instanceof AdminApiError && error.status === 401) return 'Sign in with an administrator account to manage stock.';
  if (error instanceof AdminApiError && error.status === 403) return 'Your account does not have administrator access.';
  return error instanceof Error ? error.message : 'The inventory request could not be completed.';
}

export function AdminInventoryPage() {
  const [rows, setRows] = useState<InventoryItem[]>([]);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState('');
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setFailure('');
    void fetchInventory(search, offset, controller.signal).then((page) => {
      setRows((current) => offset === 0 ? page.data : [...current, ...page.data]);
      setHasMore(page.pagination.hasMore);
      setDraft((current) => ({ ...current, ...Object.fromEntries(page.data.map((item) => [item.variantId, String(item.onHand)])) }));
    }).catch((error: unknown) => {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      setFailure(errorText(error));
      setRows([]);
      setHasMore(false);
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [search, offset, refresh]);

  function applySearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setOffset(0);
    setSearch(searchInput.trim());
  }

  async function save(item: InventoryItem) {
    const raw = draft[item.variantId] ?? String(item.onHand);
    if (!/^\d{1,7}$/.test(raw) || Number(raw) > 1_000_000) {
      setNotice('Enter an on-hand quantity from 0 to 1,000,000.');
      return;
    }
    const reason = (reasons[item.variantId] ?? '').trim();
    if (reason.length < 3 || reason.length > 500) {
      setNotice('Add an adjustment reason between 3 and 500 characters.');
      return;
    }
    setBusy(item.variantId);
    setNotice('');
    try {
      const updated = await setInventoryOnHand(item.variantId, item.onHand, Number(raw), reason);
      setRows((current) => current.map((row) => row.variantId === item.variantId ? updated : row));
      setDraft((current) => ({ ...current, [item.variantId]: String(updated.onHand) }));
      setReasons((current) => ({ ...current, [item.variantId]: '' }));
      setNotice(`Stock updated for ${item.productName} · ${item.optionValue}.`);
    } catch (error) {
      setNotice(errorText(error));
    } finally {
      setBusy('');
    }
  }

  return <main className="admin-studio admin-inventory section-pad">
    <div className="admin-studio-heading"><div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Stock, <em>with care.</em></h1><p>Review variant availability and record stock corrections with an auditable reason.</p></div><Link className="admin-studio-back" to="/admin/appointments">Studio requests ↗</Link></div>
    <nav className="admin-studio-tabs" aria-label="Admin sections"><Link to="/admin/products">Products</Link><Link to="/admin/categories">Categories</Link><Link aria-current="page" to="/admin/inventory">Inventory</Link><Link to="/admin/appointments">Studio requests</Link></nav>
    <form className="admin-studio-filters" onSubmit={applySearch}><label className="inventory-search">Search<input aria-label="Search inventory" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} maxLength={100} placeholder="Product, category or SKU" /></label><button className="button button-light" type="submit">Search stock <span>↗</span></button></form>
    {notice && <p className="admin-studio-notice" role="status">{notice}</p>}
    {failure && <div className="admin-studio-state" role="alert"><p>{failure}</p>{failure.includes('Sign in') && <Link to="/login">Sign in</Link>}<button type="button" onClick={() => { setOffset(0); setRefresh((value) => value + 1); }}>Retry</button></div>}
    {loading && <p className="admin-studio-state" role="status">Loading inventory…</p>}
    {!loading && !failure && rows.length === 0 && <div className="admin-studio-state"><h2>No variants found</h2><p>Try a different product name, category, or SKU.</p></div>}
    <div className="admin-inventory-list">{rows.map((item) => <article className="admin-inventory-card" key={item.variantId}>
      <div className="admin-inventory-title"><div><span className="admin-status">{item.categoryName} · {item.inventoryPolicy.replaceAll('_', ' ')}</span><h2>{item.productName}</h2><p>{item.optionName}: {item.optionValue} · SKU {item.sku}</p></div><Link to={`/products/${item.productSlug}`}>View product ↗</Link></div>
      <div className="inventory-counts"><p><b>On hand</b><span>{item.onHand}</span></p><p><b>Reserved</b><span>{item.reserved}</span></p><p><b>Available</b><span>{item.available}</span></p></div>
      {item.inventoryPolicy === 'tracked' ? <div className="admin-request-actions inventory-actions"><label>Set on-hand quantity<input aria-label={`On-hand quantity for ${item.productName} ${item.optionValue}`} type="number" min={item.reserved} max="1000000" step="1" value={draft[item.variantId] ?? String(item.onHand)} onChange={(event) => setDraft((current) => ({ ...current, [item.variantId]: event.target.value }))} /></label><label>Adjustment reason<input aria-label={`Adjustment reason for ${item.productName} ${item.optionValue}`} maxLength={500} value={reasons[item.variantId] ?? ''} onChange={(event) => setReasons((current) => ({ ...current, [item.variantId]: event.target.value }))} placeholder="e.g. Counted received pieces" /></label><button className="button button-red" type="button" disabled={busy === item.variantId} onClick={() => void save(item)}>{busy === item.variantId ? 'Saving…' : 'Save stock'}</button></div> : <p className="inventory-policy-note">Stock is not tracked for this made-to-order or untracked variant.</p>}
    </article>)}</div>
    {!loading && !failure && hasMore && <button className="admin-studio-more" type="button" onClick={() => setOffset((value) => value + 50)}>Load 50 more variants</button>}
  </main>;
}
