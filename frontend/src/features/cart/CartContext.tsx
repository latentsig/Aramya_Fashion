import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import type { Product } from '../products/catalog';

type CartLine = { product: Product; quantity: number };
type CartValue = { lines: CartLine[]; add: (product: Product) => void; setQuantity: (name: string, quantity: number) => void; remove: (name: string) => void; count: number; subtotal: number };
const CartContext = createContext<CartValue | null>(null);

function readCart(): Record<string, number> {
  try { return JSON.parse(localStorage.getItem('aramya-cart') ?? '{}') as Record<string, number>; }
  catch { return {}; }
}

export function CartProvider({ children }: { children: ReactNode }) {
  const [cart, setCart] = useState<Record<string, number>>(readCart);
  const save = (next: Record<string, number>) => { setCart(next); localStorage.setItem('aramya-cart', JSON.stringify(next)); };
  const value = useMemo<CartValue>(() => {
    const lines = Object.entries(cart).map(([name, quantity]) => ({ product: productsByName[name], quantity })).filter((line): line is CartLine => Boolean(line.product));
    const count = lines.reduce((total, line) => total + line.quantity, 0);
    const subtotal = lines.reduce((total, line) => total + line.product.priceRupees * line.quantity, 0);
    return {
      lines, count, subtotal,
      add: (product) => save({ ...cart, [product.name]: (cart[product.name] ?? 0) + 1 }),
      setQuantity: (name, quantity) => { const next = { ...cart }; if (quantity <= 0) delete next[name]; else next[name] = quantity; save(next); },
      remove: (name) => { const next = { ...cart }; delete next[name]; save(next); },
    };
  }, [cart]);
  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart() {
  const value = useContext(CartContext);
  if (!value) throw new Error('useCart must be used inside CartProvider');
  return value;
}

import { products } from '../products/catalog';
const productsByName = Object.fromEntries(products.map((product) => [product.name, product])) as Record<string, Product>;
