import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { useAuth } from '../features/auth/AuthContext';

const loginSchema = z.object({
  email: z.string().trim().email('Enter a valid email address.').max(254),
  password: z.string().min(1, 'Enter your password.').max(128),
});
const registerSchema = z.object({
  displayName: z.string().trim().min(1, 'Enter your name.').max(120),
  email: z.string().trim().email('Enter a valid email address.').max(254),
  password: z.string().min(12, 'Use at least 12 characters.').max(128),
});

type LoginFields = z.infer<typeof loginSchema>;
type RegisterFields = z.infer<typeof registerSchema>;

export function LoginPage() {
  const { user, loading: authLoading, login } = useAuth();
  const navigate = useNavigate();
  const [serverError, setServerError] = useState<string | null>(null);
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<LoginFields>();
  if (!authLoading && user) return <Navigate to="/account" replace />;

  const submit = handleSubmit(async (values) => {
    setServerError(null);
    const parsed = loginSchema.safeParse(values);
    if (!parsed.success) {
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === 'email' || field === 'password') setError(field, { message: issue.message });
      }
      return;
    }
    try { await login(parsed.data.email, parsed.data.password); navigate('/account', { replace: true }); }
    catch (error) { setServerError(error instanceof Error ? error.message : 'Sign in failed. Please retry.'); }
  });

  return <main className="auth-page section-pad"><section className="auth-card"><p className="eyebrow">WELCOME BACK</p><h1>Sign in to <em>your account.</em></h1><p className="auth-intro">Keep your Aramya orders and studio conversations together.</p><form className="auth-form" onSubmit={submit} noValidate>
    <label>Email address<input type="email" autoComplete="email" {...register('email')} aria-invalid={Boolean(errors.email)} />{errors.email && <span className="field-error">{errors.email.message}</span>}</label>
    <label>Password<input type="password" autoComplete="current-password" {...register('password')} aria-invalid={Boolean(errors.password)} />{errors.password && <span className="field-error">{errors.password.message}</span>}</label>
    {serverError && <p role="alert" className="auth-error">{serverError}</p>}
    <button className="button button-red" type="submit" disabled={isSubmitting}>{isSubmitting ? 'Signing in…' : 'Sign in'} <span aria-hidden="true">↗</span></button>
  </form><p className="auth-switch">New to Aramya? <Link to="/register">Create an account</Link></p></section></main>;
}

export function RegisterPage() {
  const { user, loading: authLoading, register: createAccount } = useAuth();
  const navigate = useNavigate();
  const [serverError, setServerError] = useState<string | null>(null);
  const { register, handleSubmit, setError, formState: { errors, isSubmitting } } = useForm<RegisterFields>();
  if (!authLoading && user) return <Navigate to="/account" replace />;

  const submit = handleSubmit(async (values) => {
    setServerError(null);
    const parsed = registerSchema.safeParse(values);
    if (!parsed.success) {
      for (const issue of parsed.error.issues) {
        const field = issue.path[0];
        if (field === 'email' || field === 'password' || field === 'displayName') setError(field, { message: issue.message });
      }
      return;
    }
    try { await createAccount(parsed.data.email, parsed.data.password, parsed.data.displayName); navigate('/account', { replace: true }); }
    catch (error) { setServerError(error instanceof Error ? error.message : 'Account creation failed. Please retry.'); }
  });

  return <main className="auth-page section-pad"><section className="auth-card"><p className="eyebrow">MADE PERSONAL</p><h1>Create your <em>Aramya account.</em></h1><p className="auth-intro">Save your details and keep in touch with our studio.</p><form className="auth-form" onSubmit={submit} noValidate>
    <label>Your name<input autoComplete="name" {...register('displayName')} aria-invalid={Boolean(errors.displayName)} />{errors.displayName && <span className="field-error">{errors.displayName.message}</span>}</label>
    <label>Email address<input type="email" autoComplete="email" {...register('email')} aria-invalid={Boolean(errors.email)} />{errors.email && <span className="field-error">{errors.email.message}</span>}</label>
    <label>Password<input type="password" autoComplete="new-password" {...register('password')} aria-invalid={Boolean(errors.password)} />{errors.password && <span className="field-error">{errors.password.message}</span>}<small>Use at least 12 characters.</small></label>
    {serverError && <p role="alert" className="auth-error">{serverError}</p>}
    <button className="button button-red" type="submit" disabled={isSubmitting}>{isSubmitting ? 'Creating your account…' : 'Create account'} <span aria-hidden="true">↗</span></button>
  </form><p className="auth-switch">Already have an account? <Link to="/login">Sign in</Link></p></section></main>;
}

export function AccountPage() {
  const { user, loading, logout } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  if (loading) return <main className="section-pad"><p role="status">Loading your account…</p></main>;
  if (!user) return <Navigate to="/login" replace />;

  async function signOut() {
    setSubmitting(true);
    setError(null);
    try { await logout(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Sign out failed. Please retry.'); }
    finally { setSubmitting(false); }
  }

  return <main className="account-page section-pad"><p className="eyebrow">YOUR ARAMYA ACCOUNT</p><h1>Welcome, <em>{user.displayName}.</em></h1><p className="account-email">{user.email}</p>{error && <p role="alert" className="auth-error">{error}</p>}<button type="button" className="button button-red" onClick={() => void signOut()} disabled={submitting}>{submitting ? 'Signing out…' : 'Sign out'}</button></main>;
}
