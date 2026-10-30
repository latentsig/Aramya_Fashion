import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { imageReferenceFromKey, type Product } from '../products/catalog';

type CartLine = { id: string; variantId: string; product: Product; quantity: number };
type CartValue = {
  lines: CartLine[];
  add: (product: Product) => Promise<boolean>;
  setQuantity: (variantId: string, quantity: number) => Promise<void>;
  remove: (variantId: string) => Promise<void>;
  refresh: () => Promise<void>;
  count: number;
  subtotal: number;
  loading: boolean;
  error: string | null;
};
type CartPayload = {
  lines: Array<{
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
  }>;
  count: number;
  subtotalPaise: number;
};
const CartContext = createContext<CartValue | null>(null);
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function csrfToken(): Promise<string> {
  const response = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  if (!response.ok) throw new Error('Could not prepare a secure cart request. Please retry.');
  const payload = await response.json() as { csrfToken?: string };
  if (!payload.csrfToken) throw new Error('The server did not provide a cart security token.');
  return payload.csrfToken;
}

async function requestCart(path: string, init?: RequestInit): Promise<CartPayload> {
  const method = init?.method ?? 'GET';
  const headers = new Headers(init?.headers);
  headers.set('Accept', 'application/json');
  if (method !== 'GET') {
    headers.set('Content-Type', 'application/json');
    headers.set('X-CSRF-Token', await csrfToken());
  }
  const response = await fetch(`${apiBase}/api/v1/cart${path}`, { ...init, method, headers, credentials: 'include' });
  const payload = await response.json().catch(() => null) as { data?: CartPayload; error?: { message?: string } } | null;
  if (!response.ok || !payload?.data) throw new Error(payload?.error?.message ?? 'Your bag could not be updated. Please try again.');
  return payload.data;
}

export function CartProvider({ children }: { children: ReactNode }) {
  const [snapshot, setSnapshot] = useState<CartPayload>({ lines: [], count: 0, subtotalPaise: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setSnapshot(await requestCart(''));
      setError(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Your bag could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const onAuthChange = () => { void refresh(); };
    window.addEventListener('aramya:auth-change', onAuthChange);
    return () => window.removeEventListener('aramya:auth-change', onAuthChange);
  }, [refresh]);

  const apply = async (operation: () => Promise<CartPayload>) => {
    setError(null);
    try { setSnapshot(await operation()); return true; }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Your bag could not be updated.'); return false; }
  };

  const lines = useMemo<CartLine[]>(() => snapshot.lines.map((line) => ({
    id: line.id,
    variantId: line.variantId,
    quantity: line.quantity,
    product: {
      id: line.productId,
      slug: line.slug,
      variantId: line.variantId,
      name: line.name,
      category: line.category,
      label: line.categoryName,
      image: imageReferenceFromKey(line.imageKey),
      priceRupees: line.unitPricePaise / 100,
    },
  })), [snapshot.lines]);

  const value = useMemo<CartValue>(() => ({
    lines,
    count: snapshot.count,
    subtotal: snapshot.subtotalPaise / 100,
    loading,
    error,
    add: async (product) => product.variantId
      ? apply(() => requestCart('/items', { method: 'POST', body: JSON.stringify({ variantId: product.variantId, quantity: 1 }) }))
      : (setError('Connect the live catalog to add this style to your bag.'), false),
    setQuantity: async (variantId, quantity) => {
      const line = snapshot.lines.find((item) => item.variantId === variantId);
      if (line) await apply(() => requestCart(`/items/${line.id}`, { method: 'PATCH', body: JSON.stringify({ quantity }) }));
    },
    remove: async (variantId) => {
      const line = snapshot.lines.find((item) => item.variantId === variantId);
      if (line) await apply(() => requestCart(`/items/${line.id}`, { method: 'DELETE' }));
    },
    refresh,
  }), [lines, snapshot, loading, error, refresh]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart() {
  const value = useContext(CartContext);
  if (!value) throw new Error('useCart must be used inside CartProvider');
  return value;
}
