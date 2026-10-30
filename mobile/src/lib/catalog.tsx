import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { fetchCategories, fetchProducts, type Category, type Product } from './api';

type CatalogState = {
  categories: Category[];
  products: Product[];
  loading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
};

const CatalogContext = createContext<CatalogState | null>(null);

export function CatalogProvider({ children }: { children: ReactNode }) {
  const [categories, setCategories] = useState<Category[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [nextCategories, nextProducts] = await Promise.all([fetchCategories(), fetchProducts()]);
      setCategories(nextCategories);
      setProducts(nextProducts);
      setError(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not load the collection.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    let current = true;
    Promise.all([fetchCategories(), fetchProducts()])
      .then(([nextCategories, nextProducts]) => {
        if (!current) return;
        setCategories(nextCategories);
        setProducts(nextProducts);
        setError(null);
      })
      .catch((cause: unknown) => {
        if (current) setError(cause instanceof Error ? cause.message : 'Could not load the collection.');
      })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, []);

  return <CatalogContext.Provider value={{ categories, products, loading, error, refresh }}>{children}</CatalogContext.Provider>;
}

export function useCatalog() {
  const value = useContext(CatalogContext);
  if (!value) throw new Error('useCatalog must be used inside CatalogProvider');
  return value;
}
