import { z } from 'zod';
import { AdminApiError } from './studioApi';

const orderSchema = z.object({
  orderNumber: z.string(), status: z.string(), paymentStatus: z.string(), currency: z.string(),
  totalPaise: z.number().int(), createdAt: z.string(),
});
const overviewSchema = z.object({
  activeProducts: z.number().int(), totalOrders: z.number().int(), pendingPaymentOrders: z.number().int(),
  grossCapturedPaise: z.number().int(), processedRefundsPaise: z.number().int(), netSalesPaise: z.number().int(),
  capturedPayments: z.number().int(), requestedAppointments: z.number().int(),
  openCustomOrders: z.number().int(), recentOrders: z.array(orderSchema),
});
export type AdminOverview = z.infer<typeof overviewSchema>;

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

export async function fetchAdminOverview(signal?: AbortSignal): Promise<AdminOverview> {
  const response = await fetch(`${apiBase}/api/v1/admin/overview`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  const payload = await response.json().catch(() => null) as { data?: unknown; error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(payload?.error?.message ?? 'The admin overview could not be loaded.', response.status);
  return overviewSchema.parse(payload?.data);
}
