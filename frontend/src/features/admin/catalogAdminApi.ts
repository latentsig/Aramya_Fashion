import { z } from 'zod';
import { AdminApiError } from './studioApi';

const categorySchema = z.object({
  id: z.string().uuid(), slug: z.string(), name: z.string(), subtitle: z.string(), imageKey: z.string(), sortOrder: z.number(),
  active: z.boolean(), version: z.number().int(), activeProductCount: z.number().int(), updatedAt: z.string(),
});
const variantSchema = z.object({
  id: z.string(), sku: z.string(), optionName: z.string(), optionValue: z.string(), priceAdjustmentPaise: z.number().int(), active: z.boolean(), isDefault: z.boolean(),
});
const productSummarySchema = z.object({
  id: z.string().uuid(), version: z.number().int(), sku: z.string(), slug: z.string(), name: z.string(), description: z.string(), categoryId: z.string().uuid(),
  categoryName: z.string(), pricePaise: z.number().int(), currency: z.string(), imageKey: z.string(), status: z.enum(['draft', 'active', 'archived']),
  inventoryPolicy: z.enum(['tracked', 'made_to_order', 'untracked']), defaultVariantId: z.string(), updatedAt: z.string(),
});
const productDetailSchema = productSummarySchema.extend({ variants: z.array(variantSchema) });
const imageOptionSchema = z.object({ imageKey: z.string(), label: z.string() });
const pageSchema = <T extends z.ZodTypeAny>(item: T) => z.object({
  data: z.array(item), pagination: z.object({ limit: z.number(), offset: z.number(), count: z.number(), hasMore: z.boolean() }),
});

export type AdminCategory = z.infer<typeof categorySchema>;
export type AdminProduct = z.infer<typeof productSummarySchema>;
export type AdminProductDetail = z.infer<typeof productDetailSchema>;
export type AdminProductVariant = z.infer<typeof variantSchema>;
export type ImageOption = z.infer<typeof imageOptionSchema>;
export type ProductWriteInput = {
  expectedVersion?: number; sku: string; slug: string; name: string; description: string; categoryId: string; pricePaise: number; imageKey: string;
  status: 'draft' | 'active' | 'archived'; inventoryPolicy: 'tracked' | 'made_to_order' | 'untracked';
  variants: Array<Omit<AdminProductVariant, 'id'> & { id?: string }>;
};
export type CategoryWriteInput = { expectedVersion?: number; slug: string; name: string; subtitle: string; imageKey: string; sortOrder: number; active: boolean };

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function payload(response: Response, fallback: string): Promise<unknown> {
  const data = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(data?.error?.message ?? fallback, response.status);
  return data;
}

export async function fetchAdminCategories(signal?: AbortSignal): Promise<AdminCategory[]> {
  const response = await fetch(`${apiBase}/api/v1/admin/categories`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return z.object({ data: z.array(categorySchema) }).parse(await payload(response, 'Categories could not be loaded.')).data;
}

export async function fetchAdminProducts(params: { search: string; status: string; categoryId: string; offset: number }, signal?: AbortSignal) {
  const query = new URLSearchParams({ limit: '50', offset: String(params.offset) });
  if (params.search) query.set('search', params.search);
  if (params.status) query.set('status', params.status);
  if (params.categoryId) query.set('categoryId', params.categoryId);
  const response = await fetch(`${apiBase}/api/v1/admin/products?${query}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return pageSchema(productSummarySchema).parse(await payload(response, 'Products could not be loaded.'));
}

export async function fetchAdminProduct(id: string, signal?: AbortSignal): Promise<AdminProductDetail> {
  const response = await fetch(`${apiBase}/api/v1/admin/products/${encodeURIComponent(id)}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return z.object({ data: productDetailSchema }).parse(await payload(response, 'Product could not be loaded.')).data;
}

export async function fetchCatalogImageOptions(signal?: AbortSignal): Promise<ImageOption[]> {
  const response = await fetch(`${apiBase}/api/v1/admin/catalog/image-options`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return z.object({ data: z.array(imageOptionSchema) }).parse(await payload(response, 'Catalog images could not be loaded.')).data;
}

async function mutate<T>(path: string, method: 'POST' | 'PATCH', input: unknown, schema: z.ZodType<T>, fallback: string): Promise<T> {
  const csrfResponse = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const csrf = await csrfResponse.json().catch(() => null) as { csrfToken?: string; error?: { message?: string } } | null;
  if (!csrfResponse.ok || !csrf?.csrfToken) throw new AdminApiError(csrf?.error?.message ?? 'Could not prepare a secure catalog update.', csrfResponse.status);
  const response = await fetch(`${apiBase}/api/v1${path}`, {
    method, credentials: 'include', headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrf.csrfToken }, body: JSON.stringify(input),
  });
  return schema.parse(await payload(response, fallback));
}

export async function createAdminProduct(input: ProductWriteInput): Promise<AdminProductDetail> {
  return (await mutate('/admin/products', 'POST', input, z.object({ data: productDetailSchema }), 'Product could not be created.')).data;
}
export async function updateAdminProduct(id: string, input: ProductWriteInput): Promise<AdminProductDetail> {
  return (await mutate(`/admin/products/${encodeURIComponent(id)}`, 'PATCH', input, z.object({ data: productDetailSchema }), 'Product could not be updated.')).data;
}
export async function createAdminCategory(input: CategoryWriteInput): Promise<AdminCategory> {
  return (await mutate('/admin/categories', 'POST', input, z.object({ data: categorySchema }), 'Category could not be created.')).data;
}
export async function updateAdminCategory(id: string, input: CategoryWriteInput): Promise<AdminCategory> {
  return (await mutate(`/admin/categories/${encodeURIComponent(id)}`, 'PATCH', input, z.object({ data: categorySchema }), 'Category could not be updated.')).data;
}
