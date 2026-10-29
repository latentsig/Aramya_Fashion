import { z } from 'zod';
import { AdminApiError } from './studioApi';

const itemSchema = z.object({
  variantId: z.string().uuid(), productId: z.string().uuid(), sku: z.string(), productName: z.string(), productSlug: z.string(),
  categoryName: z.string(), optionName: z.string(), optionValue: z.string(), pricePaise: z.number().int(),
  inventoryPolicy: z.enum(['tracked', 'made_to_order', 'untracked']), active: z.boolean(),
  onHand: z.number().int().nonnegative(), reserved: z.number().int().nonnegative(), available: z.number().int(), updatedAt: z.string(),
});
const pageSchema = z.object({
  data: z.array(itemSchema),
  pagination: z.object({ limit: z.number(), offset: z.number(), count: z.number(), hasMore: z.boolean() }),
});

export type InventoryItem = z.infer<typeof itemSchema>;
export type InventoryPage = z.infer<typeof pageSchema>;
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function body(response: Response) {
  const payload = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(payload?.error?.message ?? 'Inventory could not be loaded.', response.status);
  return payload;
}

export async function fetchInventory(search: string, offset: number, signal?: AbortSignal): Promise<InventoryPage> {
  const query = new URLSearchParams({ limit: '50', offset: String(offset) });
  if (search) query.set('search', search);
  const response = await fetch(`${apiBase}/api/v1/admin/inventory?${query}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return pageSchema.parse(await body(response));
}

export async function setInventoryOnHand(variantId: string, expectedOnHand: number, onHand: number, reason: string): Promise<InventoryItem> {
  const csrfResponse = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const csrfPayload = await csrfResponse.json().catch(() => null) as { csrfToken?: string; error?: { message?: string } } | null;
  if (!csrfResponse.ok || !csrfPayload?.csrfToken) throw new AdminApiError(csrfPayload?.error?.message ?? 'Could not prepare a secure stock update.', csrfResponse.status);
  const response = await fetch(`${apiBase}/api/v1/admin/inventory/${encodeURIComponent(variantId)}`, {
    method: 'PATCH', credentials: 'include', headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrfPayload.csrfToken },
    body: JSON.stringify({ expectedOnHand, onHand, reason }),
  });
  const payload = await body(response);
  return z.object({ data: itemSchema }).parse(payload).data;
}
