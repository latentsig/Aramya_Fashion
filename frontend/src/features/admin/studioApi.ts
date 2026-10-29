import { z } from 'zod';

const appointmentStatusSchema = z.enum(['requested', 'confirmed', 'completed', 'cancelled']);
const customOrderStatusSchema = z.enum(['requested', 'reviewing', 'fitting', 'quoted', 'awaiting_approval', 'approved', 'in_progress', 'completed', 'declined', 'cancelled']);
const pageSchema = <T extends z.ZodTypeAny>(item: T) => z.object({
  data: z.array(item),
  pagination: z.object({ limit: z.number(), offset: z.number(), count: z.number(), hasMore: z.boolean() }),
});

const appointmentSchema = z.object({
  id: z.string().uuid(), name: z.string(), email: z.string(), phone: z.string(), preferredAt: z.string().nullable(),
  interest: z.string(), note: z.string(), status: appointmentStatusSchema, createdAt: z.string(), updatedAt: z.string(),
});
const customOrderSchema = z.object({
  id: z.string().uuid(), name: z.string(), email: z.string(), phone: z.string(), garmentType: z.string(), occasion: z.string(),
  requirements: z.string(), measurements: z.record(z.string(), z.unknown()), quotationPaise: z.number().int().nullable(),
  currency: z.string(), status: customOrderStatusSchema, createdAt: z.string(), updatedAt: z.string(),
});

export type AppointmentRequest = z.infer<typeof appointmentSchema>;
export type CustomOrderRequest = z.infer<typeof customOrderSchema>;
export type StudioRequestPage<T> = { data: T[]; pagination: { limit: number; offset: number; count: number; hasMore: boolean } };

export class AdminApiError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function read<T>(path: string, schema: z.ZodType<T>, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${apiBase}/api/v1${path}`, { credentials: 'include', headers: { Accept: 'application/json' }, signal });
  const payload = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(payload?.error?.message ?? 'The studio queue could not be loaded.', response.status);
  return schema.parse(payload);
}

async function secureToken(): Promise<string> {
  const response = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const payload = await response.json().catch(() => null) as { csrfToken?: string; error?: { message?: string } } | null;
  if (!response.ok || !payload?.csrfToken) throw new AdminApiError(payload?.error?.message ?? 'Could not prepare a secure admin update.', response.status);
  return payload.csrfToken;
}

async function update<T>(path: string, body: unknown, schema: z.ZodType<T>): Promise<T> {
  const csrf = await secureToken();
  const response = await fetch(`${apiBase}/api/v1${path}`, {
    method: 'PATCH', credentials: 'include', headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }, body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => null) as { error?: { message?: string } } | null;
  if (!response.ok) throw new AdminApiError(payload?.error?.message ?? 'The studio request could not be updated.', response.status);
  return schema.parse(payload);
}

export function fetchAppointments(params: { status: string; search: string; offset: number }, signal?: AbortSignal): Promise<StudioRequestPage<AppointmentRequest>> {
  const query = new URLSearchParams({ limit: '50', offset: String(params.offset) });
  if (params.status) query.set('status', params.status);
  if (params.search) query.set('search', params.search);
  return read(`/admin/appointments?${query}`, pageSchema(appointmentSchema), signal);
}

export function fetchCustomOrders(params: { status: string; search: string; offset: number }, signal?: AbortSignal): Promise<StudioRequestPage<CustomOrderRequest>> {
  const query = new URLSearchParams({ limit: '50', offset: String(params.offset) });
  if (params.status) query.set('status', params.status);
  if (params.search) query.set('search', params.search);
  return read(`/admin/custom-order-requests?${query}`, pageSchema(customOrderSchema), signal);
}

export function updateAppointment(id: string, expectedStatus: string, status: string): Promise<AppointmentRequest> {
  return update(`/admin/appointments/${encodeURIComponent(id)}`, { expectedStatus, status }, z.object({ data: appointmentSchema })).then((result) => result.data);
}

export function updateCustomOrder(id: string, expectedStatus: string, status: string, quotationPaise?: number): Promise<CustomOrderRequest> {
  return update(`/admin/custom-order-requests/${encodeURIComponent(id)}`, { expectedStatus, status, ...(quotationPaise === undefined ? {} : { quotationPaise }) }, z.object({ data: customOrderSchema })).then((result) => result.data);
}
