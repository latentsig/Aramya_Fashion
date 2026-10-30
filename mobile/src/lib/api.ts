export type Category = {
  id: string;
  slug: string;
  name: string;
  subtitle: string;
  imageKey: string;
};

export type Product = {
  id: string;
  defaultVariantId: string;
  sku: string;
  slug: string;
  name: string;
  description: string | null;
  categoryId: string;
  categorySlug: string;
  categoryName: string;
  pricePaise: number;
  currency: string;
  imageKey: string;
  status: string;
  inventoryPolicy: string;
};

export type CartLine = {
  id: string;
  variantId: string;
  productId: string;
  slug: string;
  name: string;
  category: string;
  categoryName: string;
  imageKey: string;
  unitPricePaise: number;
  quantity: number;
};

export type CartSnapshot = { lines: CartLine[]; count: number; subtotalPaise: number };
export type StudioRequestResult = { id: string; status: string };
export type CheckoutQuote = { subtotalPaise: number; totalPaise: number; quoteStatus: string; paymentAvailable: boolean };
export type CheckoutOrder = { id: string; orderNumber: string; status: string; totalPaise: number };

const apiBase = (process.env.EXPO_PUBLIC_API_BASE_URL ?? '').replace(/\/$/, '');
const configuredMediaBase = process.env.EXPO_PUBLIC_MEDIA_BASE_URL ?? (apiBase ? `${apiBase}/media` : '');
const mediaBase = configuredMediaBase.replace(/\/$/, '');

function apiUrl(path: string) {
  if (!apiBase) throw new Error('Set EXPO_PUBLIC_API_BASE_URL in mobile/.env.local to connect to the Aramya API.');
  return `${apiBase}/api/v1${path}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(apiUrl(path), {
      ...init,
      credentials: 'include',
      headers: { Accept: 'application/json', ...init?.headers },
    });
  } catch {
    throw new Error('Can’t reach Aramya right now. Check your connection and API address, then retry.');
  }

  const payload = await response.json().catch(() => null) as { data?: T; error?: { message?: string } } | null;
  if (!response.ok) throw new Error(payload?.error?.message ?? `The request failed (${response.status}).`);
  if (payload && Object.hasOwn(payload, 'data')) return payload.data as T;
  return payload as T;
}

export function fetchCategories() {
  return request<Category[]>('/categories');
}

export function fetchProducts(category?: string) {
  const params = new URLSearchParams({ limit: '100' });
  if (category) params.set('category', category);
  return request<Product[]>(`/products?${params.toString()}`);
}

export function fetchProduct(slug: string) {
  return request<Product & { variants: { id: string; optionName: string; optionValue: string; available: boolean; priceAdjustmentPaise: number }[] }>(`/products/${encodeURIComponent(slug)}`);
}

export function getCart() {
  return request<CartSnapshot>('/cart');
}

async function csrfToken() {
  const result = await request<{ csrfToken: string }>('/auth/csrf');
  return result.csrfToken;
}

export function newStudioRequestKey() {
  return `studio-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}

async function submitStudioRequest<T extends StudioRequestResult>(path: string, input: Record<string, string>, idempotencyKey: string) {
  const token = await csrfToken();
  return request<T>(path, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': token,
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(input),
  });
}

export async function createDemoOrder(input: { name: string; email: string; phone: string; address: string; city: string; region: string; postalCode: string }, idempotencyKey: string) {
  const quoteToken = await csrfToken();
  const quote = await request<CheckoutQuote>('/checkout/quote', {
    method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': quoteToken }, body: JSON.stringify({}),
  });
  const token = await csrfToken();
  const order = await request<CheckoutOrder>('/checkout/orders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token, 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({
      email: input.email, phone: input.phone, customerNote: 'Sample demo order',
      address: { recipientName: input.name, phone: input.phone, line1: input.address, line2: '', city: input.city, region: input.region, postalCode: input.postalCode, countryCode: 'IN' },
    }),
  });
  return { order, quote };
}

export function createAppointment(input: { name: string; email: string; phone: string; interest: string; note: string }, idempotencyKey: string) {
  return submitStudioRequest('/appointments', input, idempotencyKey);
}

export function createCustomOrderRequest(input: { name: string; email: string; phone: string; garmentType: string; occasion: string; requirements: string }, idempotencyKey: string) {
  return submitStudioRequest('/custom-order-requests', input, idempotencyKey);
}

export async function addCartItem(variantId: string, quantity = 1) {
  const token = await csrfToken();
  return request<CartSnapshot>('/cart/items', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token },
    body: JSON.stringify({ variantId, quantity }),
  });
}

export async function setCartItemQuantity(id: string, quantity: number) {
  const token = await csrfToken();
  return request<CartSnapshot>(`/cart/items/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token },
    body: JSON.stringify({ quantity }),
  });
}

export async function removeCartItem(id: string) {
  const token = await csrfToken();
  return request<CartSnapshot>(`/cart/items/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: { 'X-CSRF-Token': token },
  });
}

export function mediaUrl(imageKey: string) {
  if (!mediaBase) return undefined;
  const legacyAsset = /^image assests[\\/]/i.test(imageKey);
  const path = (legacyAsset ? [imageKey.split(/[\\/]/).at(-1) ?? ''] : imageKey.split(/[\\/]/))
    .map((segment) => encodeURIComponent(segment))
    .join('/');
  return path ? `${mediaBase}/${path}` : undefined;
}

export function formatPrice(paise: number, currency = 'INR') {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency, maximumFractionDigits: 0 }).format(paise / 100);
}