import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import Dialog from '@mui/material/Dialog';
import { appointmentSchema, customOrderSchema, submitAppointment, submitCustomOrder, type AppointmentFields, type CustomOrderFields, type SavedStudioRequest } from '../../features/appointments/api';

type RequestMode = 'appointment' | 'custom';

function SavedConfirmation({ mode, request, onClose }: { mode: RequestMode; request: SavedStudioRequest; onClose: () => void }) {
  return <div className="studio-request-success" role="status">
    <p className="eyebrow">REQUEST SAVED</p>
    <h3>{mode === 'appointment' ? 'Thank you for reaching out.' : 'Your custom inquiry is with the studio.'}</h3>
    <p>{mode === 'appointment' ? 'This is a request, not a confirmed appointment. The studio can use the contact details you shared to arrange a convenient visit.' : 'The studio will review your garment brief and discuss fit, timing and a quotation before any order or payment is agreed.'}</p>
    <small>Request reference: {request.id}</small>
    <button className="button button-red" type="button" onClick={onClose}>Close</button>
  </div>;
}

function AppointmentRequestForm({ onSaved }: { onSaved: (request: SavedStudioRequest) => void }) {
  const idempotencyKey = useRef(crypto.randomUUID());
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<AppointmentFields>({ defaultValues: { name: '', email: '', phone: '', interest: 'Bridal or wedding wear', note: '' } });
  const submit = handleSubmit(async (values) => {
    const parsed = appointmentSchema.safeParse(values);
    if (!parsed.success) {
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === 'name' || field === 'email' || field === 'phone' || field === 'interest' || field === 'note') setError(field, { message: issue.message });
      }
      return;
    }
    try { onSaved(await submitAppointment(parsed.data, idempotencyKey.current)); }
    catch (cause) { setError('root.server', { message: cause instanceof Error ? cause.message : 'Your request could not be saved. Please retry.' }); }
  });

  return <form className="studio-request-form" onSubmit={submit} noValidate>
    <h3>Request an appointment</h3>
    <label>Your name<input autoComplete="name" {...register('name')} aria-invalid={Boolean(errors.name)} />{errors.name && <small className="field-error">{errors.name.message}</small>}</label>
    <label>Email address <span>(optional)</span><input type="email" autoComplete="email" {...register('email')} aria-invalid={Boolean(errors.email)} />{errors.email && <small className="field-error">{errors.email.message}</small>}</label>
    <label>Phone number <span>(optional)</span><input type="tel" autoComplete="tel" {...register('phone')} aria-invalid={Boolean(errors.phone)} />{errors.phone && <small className="field-error">{errors.phone.message}</small>}</label>
    <label>What are you looking for?<select {...register('interest')}><option>Bridal or wedding wear</option><option>Custom occasionwear</option><option>Ready-to-wear collection</option><option>Something else</option></select></label>
    <label>Anything you’d like the studio to know?<textarea rows={3} maxLength={1000} {...register('note')} aria-invalid={Boolean(errors.note)} />{errors.note && <small className="field-error">{errors.note.message}</small>}</label>
    {errors.root?.server && <p role="alert" className="studio-request-error">{errors.root.server.message}</p>}
    <button className="button button-red" type="submit" disabled={isSubmitting}>{isSubmitting ? 'Saving your request…' : 'Send appointment request'} <span aria-hidden="true">↗</span></button>
    <p className="form-note">A request does not reserve a time; the studio will confirm a visit with you.</p>
  </form>;
}

function CustomOrderRequestForm({ onSaved }: { onSaved: (request: SavedStudioRequest) => void }) {
  const idempotencyKey = useRef(crypto.randomUUID());
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<CustomOrderFields>({ defaultValues: { name: '', email: '', phone: '', garmentType: '', occasion: '', requirements: '' } });
  const submit = handleSubmit(async (values) => {
    const parsed = customOrderSchema.safeParse(values);
    if (!parsed.success) {
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === 'name' || field === 'email' || field === 'phone' || field === 'garmentType' || field === 'occasion' || field === 'requirements') setError(field, { message: issue.message });
      }
      return;
    }
    try { onSaved(await submitCustomOrder(parsed.data, idempotencyKey.current)); }
    catch (cause) { setError('root.server', { message: cause instanceof Error ? cause.message : 'Your request could not be saved. Please retry.' }); }
  });

  return <form className="studio-request-form" onSubmit={submit} noValidate>
    <h3>Tell us what you have in mind</h3>
    <label>Your name<input autoComplete="name" {...register('name')} aria-invalid={Boolean(errors.name)} />{errors.name && <small className="field-error">{errors.name.message}</small>}</label>
    <label>Email address <span>(optional)</span><input type="email" autoComplete="email" {...register('email')} aria-invalid={Boolean(errors.email)} />{errors.email && <small className="field-error">{errors.email.message}</small>}</label>
    <label>Phone number <span>(optional)</span><input type="tel" autoComplete="tel" {...register('phone')} aria-invalid={Boolean(errors.phone)} />{errors.phone && <small className="field-error">{errors.phone.message}</small>}</label>
    <label>Garment type<select {...register('garmentType')} aria-invalid={Boolean(errors.garmentType)}><option value="">Choose a style</option><option>Saree</option><option>Lehenga</option><option>Occasionwear</option><option>Menswear</option><option>Little ones</option><option>Other</option></select>{errors.garmentType && <small className="field-error">{errors.garmentType.message}</small>}</label>
    <label>Occasion <span>(optional)</span><input {...register('occasion')} aria-invalid={Boolean(errors.occasion)} />{errors.occasion && <small className="field-error">{errors.occasion.message}</small>}</label>
    <label>What would you like us to create?<textarea rows={3} maxLength={3000} {...register('requirements')} aria-invalid={Boolean(errors.requirements)} />{errors.requirements && <small className="field-error">{errors.requirements.message}</small>}</label>
    {errors.root?.server && <p role="alert" className="studio-request-error">{errors.root.server.message}</p>}
    <button className="button button-red" type="submit" disabled={isSubmitting}>{isSubmitting ? 'Saving your inquiry…' : 'Send custom order inquiry'} <span aria-hidden="true">↗</span></button>
    <p className="form-note">The studio will review fit and timing, then share a quotation for your approval. No payment is taken with this inquiry.</p>
  </form>;
}

export function StudioRequestDialog() {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<RequestMode>('appointment');
  const [saved, setSaved] = useState<SavedStudioRequest | null>(null);

  useEffect(() => {
    const openFromLink = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      const link = target.closest<HTMLAnchorElement>('a[href="#appointment"], a[href="#custom-order"], a[href="/#appointment"], a[href="/#custom-order"]');
      if (!link) return;
      event.preventDefault();
      setMode(link.getAttribute('href')?.endsWith('custom-order') ? 'custom' : 'appointment');
      setSaved(null);
      setOpen(true);
    };
    const openInitialHash = () => {
      if (window.location.hash === '#appointment' || window.location.hash === '#custom-order') {
        setMode(window.location.hash === '#custom-order' ? 'custom' : 'appointment');
        setOpen(true);
      }
    };
    document.addEventListener('click', openFromLink);
    openInitialHash();
    return () => document.removeEventListener('click', openFromLink);
  }, []);

  function close() { setOpen(false); setSaved(null); }

  return <Dialog open={open} onClose={close} aria-labelledby="studio-request-title" maxWidth={false} PaperProps={{ className: 'studio-request-dialog' }}>
    <button className="dialog-close" type="button" onClick={close} aria-label="Close studio request">×</button>
    <div className="appointment-dialog-inner">
      <section className="appointment-copy">
        <p className="eyebrow">A PERSONAL VISIT</p>
        <h2 id="studio-request-title">Let’s make<br /><em>time for you.</em></h2>
        <p>Tell us what you have in mind. We’ll get back to arrange a studio visit or discuss a custom creation.</p>
        <div className="studio-detail"><span>THE ARAMYA STUDIO</span><span>Ankola, Uttara Kannada, Karnataka</span><span>Visits by appointment</span></div>
      </section>
      <section className="appointment-form">
        {!saved ? <>
          <div className="studio-request-tabs" role="tablist" aria-label="Studio request type">
            <button type="button" role="tab" aria-selected={mode === 'appointment'} onClick={() => setMode('appointment')}>Studio visit</button>
            <button type="button" role="tab" aria-selected={mode === 'custom'} onClick={() => setMode('custom')}>Custom order</button>
          </div>
          {mode === 'appointment' ? <AppointmentRequestForm key="appointment" onSaved={setSaved} /> : <CustomOrderRequestForm key="custom" onSaved={setSaved} />}
        </> : <SavedConfirmation mode={mode} request={saved} onClose={close} />}
      </section>
    </div>
  </Dialog>;
}
