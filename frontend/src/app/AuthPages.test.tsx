import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '../features/auth/AuthContext';
import { AccountPage, LoginPage } from './AuthPages';

describe('customer authentication pages', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('signs in and out using CSRF-protected session endpoints', async () => {
    const user = { id: 'f9f7752d-4489-4b31-9a31-891a7cc13e9e', email: 'customer@example.test', displayName: 'Aramya Customer' };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response('', { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'login-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ user, csrfToken: 'session-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'logout-csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);
    render(<MemoryRouter initialEntries={['/login']}><AuthProvider><Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/account" element={<AccountPage />} />
    </Routes></AuthProvider></MemoryRouter>);

    fireEvent.change(await screen.findByLabelText('Email address'), { target: { value: user.email } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct-horse-battery-staple' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByRole('heading', { name: /Welcome,.*Aramya Customer/ })).toBeTruthy();

    const loginOptions = fetchMock.mock.calls[2][1];
    expect(loginOptions).toMatchObject({ credentials: 'include', method: 'POST' });
    expect(new Headers(loginOptions?.headers).get('X-CSRF-Token')).toBe('login-csrf');
    expect(JSON.parse(String(loginOptions?.body))).toEqual({ email: user.email, password: 'correct-horse-battery-staple' });

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    await waitFor(() => expect(screen.getByRole('heading', { name: /sign in to your account/i })).toBeTruthy());
    const logoutOptions = fetchMock.mock.calls[4][1];
    expect(new Headers(logoutOptions?.headers).get('X-CSRF-Token')).toBe('logout-csrf');
    expect(fetchMock).toHaveBeenCalledTimes(5);
  });
});
