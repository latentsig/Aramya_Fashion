import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { imageUrl } from '../products/catalog';
import { AdminApiError } from './studioApi';
import {
  createAdminCategory, createAdminProduct, fetchAdminCategories, fetchAdminProduct, fetchAdminProducts,
  fetchCatalogImageOptions, updateAdminCategory, updateAdminProduct, type AdminCategory, type AdminProduct,
  type AdminProductDetail, type AdminProductVariant, type CategoryWriteInput, type ImageOption, type ProductWriteInput,
} from './catalogAdminApi';
import './admin-catalog.css';

type ProductDraft = {
  sku: string; slug: string; name: string; description: string; categoryId: string; price: string; imageKey: string;
  status: 'draft' | 'active' | 'archived'; inventoryPolicy: 'tracked' | 'made_to_order' | 'untracked'; variants: AdminProductVariant[];
};
type CategoryDraft = { slug: string; name: string; subtitle: string; imageKey: string; sortOrder: string; active: boolean };

const blankProduct: ProductDraft = {
  sku: '', slug: '', name: '', description: '', categoryId: '', price: '', imageKey: '', status: 'draft', inventoryPolicy: 'made_to_order',
  variants: [{ id: '', sku: '', optionName: 'Fit', optionValue: 'Made to measure', priceAdjustmentPaise: 0, active: true, isDefault: true }],
};
const blankCategory: CategoryDraft = { slug: '', name: '', subtitle: '', imageKey: '', sortOrder: '0', active: true };

function money(paise: number) { return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 2 }).format(paise / 100); }
function toPaise(value: string): number | null {
  if (!/^\d{1,9}(?:\.\d{1,2})?$/.test(value)) return null;
  const [rupees, decimal = ''] = value.split('.');
  const paise = Number(rupees) * 100 + Number(decimal.padEnd(2, '0'));
  return Number.isSafeInteger(paise) && paise <= 100_000_000_000 ? paise : null;
}
function priceToRupees(paise: number) { return (paise / 100).toFixed(2); }
function assetURL(key: string) { return imageUrl(key.split(/[\\/]/).at(-1) ?? key); }
function friendly(error: unknown) {
  if (error instanceof AdminApiError && error.status === 401) return 'Sign in with an administrator account to manage the catalog.';
  if (error instanceof AdminApiError && error.status === 403) return 'Your account does not have administrator access.';
  if (error instanceof AdminApiError && error.status === 409) return error.message;
  return error instanceof Error ? error.message : 'The catalog request could not be completed.';
}
function adminTabs(active: 'products' | 'categories') {
  return <nav className="admin-studio-tabs" aria-label="Admin sections">
    <Link aria-current={active === 'products' ? 'page' : undefined} to="/admin/products">Products</Link>
    <Link aria-current={active === 'categories' ? 'page' : undefined} to="/admin/categories">Categories</Link>
    <Link to="/admin/inventory">Inventory</Link><Link to="/admin/appointments">Studio requests</Link>
  </nav>;
}

function toProductDraft(product: AdminProductDetail): ProductDraft {
  return { sku: product.sku, slug: product.slug, name: product.name, description: product.description, categoryId: product.categoryId,
    price: priceToRupees(product.pricePaise), imageKey: product.imageKey, status: product.status, inventoryPolicy: product.inventoryPolicy, variants: product.variants };
}

export function AdminProductsPage() {
  const [products, setProducts] = useState<AdminProduct[]>([]);
  const [categories, setCategories] = useState<AdminCategory[]>([]);
  const [images, setImages] = useState<ImageOption[]>([]);
  const [draft, setDraft] = useState<ProductDraft>(blankProduct);
  const [editing, setEditing] = useState<AdminProductDetail | null>(null);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState('');
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([fetchAdminCategories(controller.signal), fetchCatalogImageOptions(controller.signal)]).then(([categoryData, imageData]) => {
      setCategories(categoryData); setImages(imageData);
    }).catch((error: unknown) => { if (!(error instanceof DOMException && error.name === 'AbortError')) setFailure(friendly(error)); });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setFailure('');
    void fetchAdminProducts({ search, status, categoryId, offset }, controller.signal).then((page) => {
      setProducts((current) => offset === 0 ? page.data : [...current, ...page.data]); setHasMore(page.pagination.hasMore);
    }).catch((error: unknown) => { if (error instanceof DOMException && error.name === 'AbortError') return; setProducts([]); setHasMore(false); setFailure(friendly(error)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [search, status, categoryId, offset, refresh]);

  function applySearch(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setOffset(0); setSearch(searchInput.trim()); }
  function newProduct() { setEditing(null); setDraft({ ...blankProduct, variants: blankProduct.variants.map((variant) => ({ ...variant })) }); setFormOpen(true); setNotice(''); }

  async function editProduct(product: AdminProduct) {
    setFailure(''); setNotice('');
    try { const detail = await fetchAdminProduct(product.id); setEditing(detail); setDraft(toProductDraft(detail)); setFormOpen(true); }
    catch (error) { setFailure(friendly(error)); }
  }

  function updateVariant(index: number, field: keyof AdminProductVariant, value: string | boolean | number) {
    setDraft((current) => ({ ...current, variants: current.variants.map((variant, at) => at === index ? { ...variant, [field]: value } : variant) }));
  }

  async function saveProduct(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const pricePaise = toPaise(draft.price);
    if (pricePaise === null) { setNotice('Enter a valid price in rupees with up to two decimal places.'); return; }
    if (!draft.categoryId || !draft.imageKey) { setNotice('Choose an existing category and catalog image.'); return; }
    if (draft.variants.filter((variant) => variant.isDefault).length !== 1 || !draft.variants.some((variant) => variant.isDefault && variant.active)) {
      setNotice('Choose exactly one active default variant.'); return;
    }
    const input: ProductWriteInput = {
      ...(editing ? { expectedVersion: editing.version } : {}), sku: draft.sku, slug: draft.slug, name: draft.name, description: draft.description,
      categoryId: draft.categoryId, pricePaise, imageKey: draft.imageKey, status: draft.status, inventoryPolicy: draft.inventoryPolicy,
      variants: draft.variants.map(({ id, ...variant }) => ({ ...(id ? { id } : {}), ...variant })),
    };
    setSaving(true); setNotice(''); setFailure('');
    try {
      if (editing) await updateAdminProduct(editing.id, input); else await createAdminProduct(input);
      setFormOpen(false); setEditing(null); setNotice(editing ? 'Product changes saved.' : 'Product created.'); setOffset(0); setRefresh((value) => value + 1);
    } catch (error) { setNotice(friendly(error)); }
    finally { setSaving(false); }
  }

  return <main className="admin-studio admin-catalog section-pad">
    <div className="admin-studio-heading"><div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Catalog, <em>considered.</em></h1><p>Create and curate real catalog entries with versioned edits and preserved variant history.</p></div><button className="button button-red" type="button" onClick={newProduct}>Add product <span>↗</span></button></div>
    {adminTabs('products')}
    <form className="admin-studio-filters admin-catalog-filters" onSubmit={applySearch}>
      <label>Product search<input aria-label="Search products" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} maxLength={100} placeholder="Name, SKU, URL or category" /></label>
      <label>Status<select aria-label="Filter products by status" value={status} onChange={(event) => { setStatus(event.target.value); setOffset(0); }}><option value="">All statuses</option><option value="draft">Draft</option><option value="active">Active</option><option value="archived">Archived</option></select></label>
      <label>Category<select aria-label="Filter products by category" value={categoryId} onChange={(event) => { setCategoryId(event.target.value); setOffset(0); }}><option value="">All categories</option>{categories.map((category) => <option value={category.id} key={category.id}>{category.name}{category.active ? '' : ' · inactive'}</option>)}</select></label>
      <button className="button button-light" type="submit">Search catalog <span>↗</span></button>
    </form>
    {failure && <p className="admin-studio-notice error" role="alert">{failure}</p>}{notice && !formOpen && <p className="admin-studio-notice success" role="status">{notice}</p>}
    {loading && <p className="admin-studio-state" role="status">Loading products…</p>}
    {!loading && !failure && products.length === 0 && <div className="admin-studio-state"><h2>No products found</h2><p>Adjust the filters or add a product to the catalog.</p></div>}
    <div className="admin-product-list">{products.map((product) => <article className="admin-product-row" key={product.id}>
      <img src={assetURL(product.imageKey)} alt="" loading="lazy" /><div className="admin-product-info"><span className={`admin-status status-${product.status}`}>{product.status}</span><h2>{product.name}</h2><p>{product.categoryName} · SKU {product.sku}</p><small>{product.slug} · {product.inventoryPolicy.replaceAll('_', ' ')} · v{product.version}</small></div>
      <div className="admin-product-price"><b>{money(product.pricePaise)}</b><button className="admin-studio-back" type="button" onClick={() => void editProduct(product)}>Edit product</button></div>
    </article>)}</div>
    {!loading && hasMore && <button className="admin-studio-more" type="button" onClick={() => setOffset((value) => value + 50)}>Load more products</button>}

    {formOpen && <div className="admin-catalog-overlay" role="presentation"><section className="admin-catalog-panel" role="dialog" aria-modal="true" aria-labelledby="product-form-heading">
      <div className="admin-catalog-panel-head"><div><p className="eyebrow">CATALOG EDITION</p><h2 id="product-form-heading">{editing ? 'Edit product' : 'Create product'}</h2></div><button type="button" aria-label="Close product form" onClick={() => { setFormOpen(false); setNotice(''); }}>×</button></div>
      {notice && <p className="admin-studio-notice error" role="alert">{notice}</p>}
      <form className="admin-catalog-form" onSubmit={(event) => void saveProduct(event)}>
        <div className="admin-form-grid"><label>Product name<input required minLength={2} maxLength={160} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
          <label>URL slug<input required pattern="[a-z0-9]+(-[a-z0-9]+)*" maxLength={160} value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value })} /></label>
          <label>Product SKU<input required pattern="[A-Za-z0-9][A-Za-z0-9_-]{1,63}" maxLength={64} value={draft.sku} onChange={(event) => setDraft({ ...draft, sku: event.target.value })} /></label>
          <label>Category<select required value={draft.categoryId} onChange={(event) => setDraft({ ...draft, categoryId: event.target.value })}><option value="">Choose category</option>{categories.map((category) => <option key={category.id} value={category.id}>{category.name}{category.active ? '' : ' · inactive'}</option>)}</select></label>
          <label>Price (₹)<input required inputMode="decimal" type="number" min="0" max="1000000000" step="0.01" value={draft.price} onChange={(event) => setDraft({ ...draft, price: event.target.value })} /></label>
          <label>Publication<select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as ProductDraft['status'] })}><option value="draft">Draft</option><option value="active">Active</option><option value="archived">Archived</option></select></label>
          <label>Inventory policy<select value={draft.inventoryPolicy} onChange={(event) => setDraft({ ...draft, inventoryPolicy: event.target.value as ProductDraft['inventoryPolicy'] })}><option value="made_to_order">Made to order</option><option value="tracked">Tracked stock</option><option value="untracked">Untracked</option></select></label>
          <label>Primary catalog image<select required value={draft.imageKey} onChange={(event) => setDraft({ ...draft, imageKey: event.target.value })}><option value="">Choose existing asset</option>{images.map((image) => <option key={image.imageKey} value={image.imageKey}>{image.label}</option>)}</select></label>
        </div>
        {draft.imageKey && <div className="admin-selected-image"><img src={assetURL(draft.imageKey)} alt="Selected catalog asset preview" /><span>Existing catalog asset · {draft.imageKey}</span></div>}
        <label>Description<textarea rows={4} maxLength={5000} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label>
        <fieldset className="admin-variant-editor"><legend>Variants · {draft.variants.length}/50</legend><p>Keep existing variants in the form. Deactivate a variant to hide it from new orders while preserving order history.</p>
          {draft.variants.map((variant, index) => <div className="admin-variant-row" key={variant.id || `new-${index}`}>
            <label>Variant SKU<input required pattern="[A-Za-z0-9][A-Za-z0-9_-]{1,63}" maxLength={64} value={variant.sku} onChange={(event) => updateVariant(index, 'sku', event.target.value)} /></label>
            <label>Option name<input required maxLength={60} value={variant.optionName} onChange={(event) => updateVariant(index, 'optionName', event.target.value)} /></label>
            <label>Option value<input required maxLength={100} value={variant.optionValue} onChange={(event) => updateVariant(index, 'optionValue', event.target.value)} /></label>
            <label>Price adjustment (₹)<input inputMode="decimal" type="number" min="-1000000000" max="1000000000" step="0.01" value={priceToRupees(variant.priceAdjustmentPaise)} onChange={(event) => { const parsed = /^-?\d{1,9}(?:\.\d{1,2})?$/.test(event.target.value) ? Math.round(Number(event.target.value) * 100) : variant.priceAdjustmentPaise; updateVariant(index, 'priceAdjustmentPaise', parsed); }} /></label>
            <label className="admin-check"><input type="checkbox" checked={variant.active} onChange={(event) => updateVariant(index, 'active', event.target.checked)} /> Active</label>
            <label className="admin-check"><input type="radio" name="defaultVariant" checked={variant.isDefault} onChange={() => setDraft((current) => ({ ...current, variants: current.variants.map((item, at) => ({ ...item, isDefault: at === index })) }))} /> Default</label>
          </div>)}
          <button className="admin-add-variant" type="button" disabled={draft.variants.length >= 50} onClick={() => setDraft((current) => ({ ...current, variants: [...current.variants, { id: '', sku: '', optionName: 'Size', optionValue: '', priceAdjustmentPaise: 0, active: true, isDefault: false }] }))}>Add variant</button>
        </fieldset>
        {editing && <p className="admin-version-note">Editing version {editing.version}. The server rejects stale edits and retains historical order records.</p>}
        <div className="admin-catalog-actions"><button className="button button-red" type="submit" disabled={saving || categories.length === 0 || images.length === 0}>{saving ? 'Saving…' : editing ? 'Save product' : 'Create product'}</button><button className="button button-light" type="button" disabled={saving} onClick={() => { setFormOpen(false); setNotice(''); }}>Cancel</button></div>
        {images.length === 0 && <p className="admin-version-note">No approved existing catalog images are available. R2 media uploads are not configured yet.</p>}
      </form>
    </section></div>}
  </main>;
}

function toCategoryDraft(category: AdminCategory): CategoryDraft { return { slug: category.slug, name: category.name, subtitle: category.subtitle, imageKey: category.imageKey, sortOrder: String(category.sortOrder), active: category.active }; }

export function AdminCategoriesPage() {
  const [categories, setCategories] = useState<AdminCategory[]>([]);
  const [images, setImages] = useState<ImageOption[]>([]);
  const [editing, setEditing] = useState<AdminCategory | null>(null);
  const [draft, setDraft] = useState<CategoryDraft>(blankCategory);
  const [formOpen, setFormOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState('');
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setFailure('');
    void Promise.all([fetchAdminCategories(controller.signal), fetchCatalogImageOptions(controller.signal)]).then(([categoryData,imageData]) => { setCategories(categoryData); setImages(imageData); })
      .catch((error: unknown) => { if (!(error instanceof DOMException && error.name === 'AbortError')) setFailure(friendly(error)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [refresh]);

  function newCategory() { setEditing(null); setDraft({ ...blankCategory }); setFormOpen(true); setNotice(''); }
  function editCategory(category: AdminCategory) { setEditing(category); setDraft(toCategoryDraft(category)); setFormOpen(true); setNotice(''); }

  async function saveCategory(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const sortOrder = Number(draft.sortOrder);
    if (!Number.isInteger(sortOrder) || sortOrder < 0 || sortOrder > 1_000_000) { setNotice('Sort order must be a whole number from 0 to 1,000,000.'); return; }
    if (!draft.imageKey) { setNotice('Choose an image from the existing catalog asset library.'); return; }
    const input: CategoryWriteInput = { ...(editing ? { expectedVersion: editing.version } : {}), slug: draft.slug, name: draft.name, subtitle: draft.subtitle, imageKey: draft.imageKey, sortOrder, active: draft.active };
    setSaving(true); setNotice(''); setFailure('');
    try {
      if (editing) await updateAdminCategory(editing.id,input); else await createAdminCategory(input);
      setFormOpen(false); setNotice(editing ? 'Category changes saved.' : 'Category created.'); setRefresh((value) => value + 1);
    } catch (error) { setNotice(friendly(error)); }
    finally { setSaving(false); }
  }

  return <main className="admin-studio admin-catalog section-pad">
    <div className="admin-studio-heading"><div><p className="eyebrow">ARAMYA · STUDIO TEAM</p><h1>Collections, <em>curated.</em></h1><p>Manage collection names, ordering, visibility, and their existing brand imagery.</p></div><button className="button button-red" type="button" onClick={newCategory}>Add category <span>↗</span></button></div>
    {adminTabs('categories')}
    {failure && <p className="admin-studio-notice error" role="alert">{failure}</p>}{notice && !formOpen && <p className="admin-studio-notice success" role="status">{notice}</p>}
    {loading && <p className="admin-studio-state" role="status">Loading categories…</p>}
    {!loading && !failure && categories.length === 0 && <div className="admin-studio-state"><h2>No categories found</h2><p>Create the first collection when your image library is available.</p></div>}
    <div className="admin-category-list">{categories.map((category) => <article className="admin-category-row" key={category.id}>
      <img src={assetURL(category.imageKey)} alt="" loading="lazy" /><div><span className={`admin-status ${category.active ? 'status-active' : 'status-archived'}`}>{category.active ? 'Active' : 'Inactive'}</span><h2>{category.name}</h2><p>{category.subtitle}</p><small>/{category.slug} · order {category.sortOrder} · {category.activeProductCount} active products · v{category.version}</small></div><button className="admin-studio-back" type="button" onClick={() => editCategory(category)}>Edit category</button>
    </article>)}</div>
    {formOpen && <div className="admin-catalog-overlay" role="presentation"><section className="admin-catalog-panel" role="dialog" aria-modal="true" aria-labelledby="category-form-heading">
      <div className="admin-catalog-panel-head"><div><p className="eyebrow">COLLECTION EDITION</p><h2 id="category-form-heading">{editing ? 'Edit category' : 'Create category'}</h2></div><button type="button" aria-label="Close category form" onClick={() => setFormOpen(false)}>×</button></div>
      {notice && <p className="admin-studio-notice error" role="alert">{notice}</p>}
      <form className="admin-catalog-form" onSubmit={(event) => void saveCategory(event)}>
        <div className="admin-form-grid"><label>Category name<input required minLength={2} maxLength={120} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
          <label>URL slug<input required pattern="[a-z0-9]+(-[a-z0-9]+)*" maxLength={120} value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value })} /></label>
          <label>Subtitle<input maxLength={300} value={draft.subtitle} onChange={(event) => setDraft({ ...draft, subtitle: event.target.value })} /></label>
          <label>Sort order<input required type="number" min="0" max="1000000" step="1" value={draft.sortOrder} onChange={(event) => setDraft({ ...draft, sortOrder: event.target.value })} /></label>
          <label>Collection image<select required value={draft.imageKey} onChange={(event) => setDraft({ ...draft, imageKey: event.target.value })}><option value="">Choose existing asset</option>{images.map((image) => <option key={image.imageKey} value={image.imageKey}>{image.label}</option>)}</select></label>
          <label className="admin-check"><input type="checkbox" checked={draft.active} disabled={Boolean(editing && editing.activeProductCount > 0)} onChange={(event) => setDraft({ ...draft, active: event.target.checked })} /> Collection is visible</label>
        </div>
        {draft.imageKey && <div className="admin-selected-image"><img src={assetURL(draft.imageKey)} alt="Selected collection image preview" /><span>{draft.imageKey}</span></div>}
        {editing && editing.activeProductCount > 0 && <p className="admin-version-note">Archive or move the {editing.activeProductCount} active products before deactivating this collection. Product history is retained.</p>}
        <div className="admin-catalog-actions"><button className="button button-red" type="submit" disabled={saving || images.length === 0}>{saving ? 'Saving…' : editing ? 'Save category' : 'Create category'}</button><button className="button button-light" type="button" disabled={saving} onClick={() => setFormOpen(false)}>Cancel</button></div>
        {images.length === 0 && <p className="admin-version-note">No approved existing catalog images are available. R2 media uploads are not configured yet.</p>}
      </form>
    </section></div>}
  </main>;
}
