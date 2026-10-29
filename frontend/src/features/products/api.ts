import { z } from 'zod';
import { imageUrl, type Product } from './catalog';

const productSchema = z.object({
  id: z.string().uuid(),
  defaultVariantId: z.string().uuid(),
  sku: z.string(),
  slug: z.string(),
  name: z.string(),
  description: z.string().nullable(),
  categoryId: z.string().uuid(),
  categorySlug: z.string(),
  categoryName: z.string(),
  pricePaise: z.number().int().nonnegative(),
  currency: z.string().length(3),
  imageKey: z.string(),
  status: z.string(),
  inventoryPolicy: z.string(),
});

const productResponseSchema = z.object({ data: z.array(productSchema) });
const variantSchema = z.object({
  id: z.string().uuid(),
  sku: z.string(),
  optionName: z.string(),
  optionValue: z.string(),
  priceAdjustmentPaise: z.number().int(),
  isDefault: z.boolean(),
  available: z.boolean(),
});
const imageSchema = z.object({ imageKey: z.string(), altText: z.string() });
const productDetailResponseSchema = z.object({
  data: productSchema.extend({ variants: z.array(variantSchema), images: z.array(imageSchema) }),
});

export type CatalogProduct = Product & { id: string; slug: string; sku: string };
export type CatalogVariant = z.infer<typeof variantSchema>;
export type CatalogProductDetail = CatalogProduct & {
  description: string | null;
  inventoryPolicy: string;
  variants: CatalogVariant[];
  images: z.infer<typeof imageSchema>[];
};

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

export async function fetchCatalogProducts(category?: string, signal?: AbortSignal): Promise<CatalogProduct[]> {
  const url = new URL(`${apiBase}/api/v1/products`, window.location.origin);
  if (category) url.searchParams.set('category', category);
  url.searchParams.set('limit', '100');
  const response = await fetch(url, { signal, headers: { Accept: 'application/json' } });
  if (!response.ok) throw new Error(`Catalog request failed (${response.status})`);
  const payload = productResponseSchema.parse(await response.json());
  return payload.data.map((product) => ({
    id: product.id,
    variantId: product.defaultVariantId,
    slug: product.slug,
    sku: product.sku,
    name: product.name,
    category: product.categorySlug,
    label: product.categoryName,
    image: product.imageKey.split(/[\\/]/).at(-1) ?? product.imageKey,
    priceRupees: product.pricePaise / 100,
  }));
}

export async function fetchCatalogProduct(slug: string, signal?: AbortSignal): Promise<CatalogProductDetail> {
  const url = `${apiBase}/api/v1/products/${encodeURIComponent(slug)}`;
  const response = await fetch(url, { signal, headers: { Accept: 'application/json' } });
  if (!response.ok) throw new Error(response.status === 404 ? 'This style is no longer available.' : `Product request failed (${response.status})`);
  const { data } = productDetailResponseSchema.parse(await response.json());
  return {
    id: data.id,
    slug: data.slug,
    sku: data.sku,
    variantId: data.defaultVariantId,
    name: data.name,
    category: data.categorySlug,
    label: data.categoryName,
    image: data.imageKey.split(/[\\/]/).at(-1) ?? data.imageKey,
    priceRupees: data.pricePaise / 100,
    description: data.description,
    inventoryPolicy: data.inventoryPolicy,
    variants: data.variants,
    images: data.images,
  };
}

export { imageUrl };
