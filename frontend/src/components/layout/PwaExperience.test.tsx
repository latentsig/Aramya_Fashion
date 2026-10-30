import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { PwaExperience } from './PwaExperience';

vi.mock('virtual:pwa-register', () => ({ registerSW: vi.fn(() => vi.fn()) }));

describe('PWA experience', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
  });

  it('explains that cart and payment need a connection while offline', () => {
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false });
    render(<PwaExperience />);

    expect(screen.getByRole('status').textContent).toMatch(/bag updates, checkout and payments need a connection/i);
  });

  it('offers installation only after the browser supplies its install prompt', async () => {
    render(<PwaExperience />);
    const prompt = vi.fn().mockResolvedValue(undefined);
    const event = Object.assign(new Event('beforeinstallprompt', { cancelable: true }), {
      prompt,
      userChoice: Promise.resolve({ outcome: 'accepted' as const, platform: 'web' }),
    });

    window.dispatchEvent(event);
    fireEvent.click(await screen.findByRole('button', { name: 'Install Aramya' }));

    await waitFor(() => expect(prompt).toHaveBeenCalledOnce());
  });
});
