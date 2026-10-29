import { z } from 'zod';

const statusSchema = z.enum(['pending_payment', 'paid', 'confirmed', 'processing', 'ready_to_ship', 'shipped', 'delivered', 'cancelled', 'refund_pending', 'refunded']);
const summarySchema = z.object({
  id: z.string().uuid(),
  orderNumber: z.string(),
  status: statusSchema,
  currency: z.string().length(3),
  subtotalPaise: z.number().int().nonnegative(),
  taxPaise: z.number().int().nonnegative(),
  shippingPaise: z.number().int().nonnegative(),
  discountPaise: z.number().int().nonnegative(),
  totalPaise: z.number().int().nonnegative(),
  createdAt: z.string(),
});
const paginationSchema = z.object({
  limit: z.number().int().positive(),
  offset: z.number().int().nonnegative(),
  count: z.number().int().nonnegative(),
  hasMore: z.boolean(),
});
const listResponseSchema = z.object({ data: z.array(summarySchema), pagination: paginationSchema });
const addressSchema = z.object({
  recipientName: z.string(),
  phone: z.string(),
  line1: z.string(),
  line2: z.string(),
  city: z.string(),
  region: z.string(),
  postalCode: z.string(),
  countryCode: z.string().length(2),
});
const itemSchema = z.object({
  id: z.string().uuid(),
  productId: z.string().uuid(),
  variantId: z.string().uuid(),
  productName: z.string(),
  productSku: z.string(),
  variantDescription: z.string(),
  quantity: z.number().int().positive(),
  unitPricePaise: z.number().int().nonnegative(),
  taxPaise: z.number().int().nonnegative(),
  lineTotalPaise: z.number().int().nonnegative(),
});
const detailResponseSchema = z.object({
  data: summarySchema.extend({
    shippingAddress: addressSchema,
    customerNote: z.string(),
    items: z.array(itemSchema),
  }),
});

export type OrderSummary = z.infer<typeof summarySchema>;
export type OrderDetail = z.infer<typeof detailResponseSchema>['data'];
export type OrderPage = z.infer<typeof listResponseSchema>;

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function getJSON<T>(path: string, schema: z.ZodType<T>, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${apiBase}/api/v1${path}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
    signal,
  });
  const payload = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new Error(payload?.error?.message ?? 'Your order information could not be loaded. Please retry.');
  return schema.parse(payload);
}

export function fetchOrders(offset: number, signal?: AbortSignal): Promise<OrderPage> {
  const query = new URLSearchParams({ limit: '20', offset: String(offset) });
  return getJSON(`/orders?${query}`, listResponseSchema, signal);
}

export function fetchOrder(id: string, signal?: AbortSignal): Promise<OrderDetail> {
  if (!z.string().uuid().safeParse(id).success) throw new Error('Order ID is invalid.');
  return getJSON(`/orders/${encodeURIComponent(id)}`, detailResponseSchema, signal).then(({ data }) => data);
}
