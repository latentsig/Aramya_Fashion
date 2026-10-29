import { z } from 'zod';

const contactFields = {
  name: z.string().trim().min(1, 'Enter your name.').max(120, 'Name is too long.'),
  email: z.union([z.literal(''), z.string().trim().email('Enter a valid email address.').max(254)]).default(''),
  phone: z.string().trim().max(32, 'Phone number is too long.').default(''),
};
const requestContactSchema = z.object(contactFields).superRefine((value, ctx) => {
  const phoneDigits = value.phone.replace(/\D/g, '');
  if (value.email === '' && phoneDigits.length < 7) {
    ctx.addIssue({ code: 'custom', path: ['phone'], message: 'Provide an email address or a phone number the studio can reach.' });
  }
  if (value.phone !== '' && (!/^[+()\d .-]+$/.test(value.phone) || phoneDigits.length > 15 || phoneDigits.length < 7)) {
    ctx.addIssue({ code: 'custom', path: ['phone'], message: 'Enter a valid phone number.' });
  }
});

export const appointmentSchema = requestContactSchema.extend({
  interest: z.string().trim().max(120).default('Bridal or wedding wear'),
  note: z.string().trim().max(1000, 'Please keep your note under 1,000 characters.').default(''),
});

export const customOrderSchema = requestContactSchema.extend({
  garmentType: z.string().trim().min(1, 'Choose the kind of garment.').max(120),
  occasion: z.string().trim().max(160, 'Occasion is too long.').default(''),
  requirements: z.string().trim().max(3000, 'Please keep your request under 3,000 characters.').default(''),
});

export type AppointmentFields = z.infer<typeof appointmentSchema>;
export type CustomOrderFields = z.infer<typeof customOrderSchema>;
export type SavedStudioRequest = { id: string; status: 'requested'; createdAt: string };

const savedRequestSchema = z.object({ data: z.object({
  id: z.string().uuid(), status: z.literal('requested'), createdAt: z.string().datetime(),
}) });
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function sendRequest(path: string, body: AppointmentFields | CustomOrderFields, idempotencyKey: string): Promise<SavedStudioRequest> {
  const csrfResponse = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const csrfPayload = await csrfResponse.json().catch(() => null) as { csrfToken?: string } | null;
  if (!csrfResponse.ok || !csrfPayload?.csrfToken) throw new Error('Could not prepare a secure request. Please retry.');

  const response = await fetch(`${apiBase}/api/v1${path}`, {
    method: 'POST', credentials: 'include',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': csrfPayload.csrfToken, 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => null) as { data?: unknown; error?: { message?: string } } | null;
  if (!response.ok) throw new Error(payload?.error?.message ?? 'Your request could not be saved. Please retry.');
  const parsed = savedRequestSchema.safeParse(payload);
  if (!parsed.success) throw new Error('The studio saved your request but returned an unexpected confirmation. Please contact the studio before submitting again.');
  return parsed.data.data;
}

export function submitAppointment(fields: AppointmentFields, idempotencyKey: string) {
  return sendRequest('/appointments', fields, idempotencyKey);
}

export function submitCustomOrder(fields: CustomOrderFields, idempotencyKey: string) {
  return sendRequest('/custom-order-requests', fields, idempotencyKey);
}
