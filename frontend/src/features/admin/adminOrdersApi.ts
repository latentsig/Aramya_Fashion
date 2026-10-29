import { z } from 'zod';
import { AdminApiError } from './studioApi';

const statusSchema = z.enum(['pending_payment', 'paid', 'confirmed', 'processing', 'ready_to_ship', 'shipped', 'delivered', 'cancelled', 'refund_pending', 'refunded']);
const shipmentStatusSchema = z.enum(['unfulfilled', 'ready', 'shipped', 'delivered', 'returned']);
const adminOrderSchema = z.object({
  id: z.string().uuid(), version: z.number().int(), orderNumber: z.string(), status: statusSchema, currency: z.string(),
  subtotalPaise: z.number().int(), taxPaise: z.number().int(), shippingPaise: z.number().int(), discountPaise: z.number().int(), totalPaise: z.number().int(),
  customerName: z.string(), customerEmail: z.string(), customerPhone: z.string(), paymentStatus: z.string(), refundStatus: z.string(), refundedPaise: z.number().int(), shipmentStatus: shipmentStatusSchema,
  shipmentProvider: z.string(), trackingNumber: z.string(), trackingUrl: z.string(), createdAt: z.string(), updatedAt: z.string(),
});
const detailSchema = adminOrderSchema.extend({
  shippingAddress: z.record(z.string(), z.unknown()), customerNote: z.string(),
  items: z.array(z.object({ id: z.string().uuid(), productName: z.string(), productSku: z.string(), variantDescription: z.string(), quantity: z.number().int(), unitPricePaise: z.number().int(), taxPaise: z.number().int(), lineTotalPaise: z.number().int() })),
  events: z.array(z.object({ fromStatus: z.string(), toStatus: z.string(), reason: z.string(), actorName: z.string(), createdAt: z.string() })),
});
const pageSchema = z.object({ data: z.array(adminOrderSchema), pagination: z.object({ limit: z.number(), offset: z.number(), count: z.number(), hasMore: z.boolean() }) });
export type AdminOrder = z.infer<typeof adminOrderSchema>;
export type AdminOrderDetail = z.infer<typeof detailSchema>;
export type OrderWrite = { expectedVersion: number; status: AdminOrder['status']; reason: string; shipmentProvider?: string; trackingNumber?: string; trackingUrl?: string };

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');
async function read(response: Response, fallback: string): Promise<unknown> {
  const data = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(data?.error?.message ?? fallback, response.status);
  return data;
}
export async function fetchAdminOrders(params: { search: string; status: string; offset: number }, signal?: AbortSignal) {
  const query = new URLSearchParams({ limit: '50', offset: String(params.offset) });
  if (params.search) query.set('search', params.search);
  if (params.status) query.set('status', params.status);
  const response = await fetch(`${apiBase}/api/v1/admin/orders?${query}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return pageSchema.parse(await read(response, 'Orders could not be loaded.'));
}
export async function fetchAdminOrder(id: string, signal?: AbortSignal): Promise<AdminOrderDetail> {
  const response = await fetch(`${apiBase}/api/v1/admin/orders/${encodeURIComponent(id)}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  return z.object({ data: detailSchema }).parse(await read(response, 'Order could not be loaded.')).data;
}
export async function updateAdminOrder(id: string, input: OrderWrite): Promise<AdminOrderDetail> {
  const csrfResponse = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const csrf = await csrfResponse.json().catch(() => null) as { csrfToken?: string; error?: { message?: string } } | null;
  if (!csrfResponse.ok || !csrf?.csrfToken) throw new AdminApiError(csrf?.error?.message ?? 'Could not prepare a secure order update.', csrfResponse.status);
  const response = await fetch(`${apiBase}/api/v1/admin/orders/${encodeURIComponent(id)}`, {
    method: 'PATCH', credentials: 'include', headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrf.csrfToken }, body: JSON.stringify(input),
  });
  return z.object({ data: detailSchema }).parse(await read(response, 'Order could not be updated.')).data;
}
