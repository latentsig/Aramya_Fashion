import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CartProvider, useCart } from './CartContext';
import type { Product } from '../products/catalog';

const product: Product = {
  id: 'f9f7752d-4489-4b31-9a31-891a7cc13e9e',
  slug: 'heirloom-saree',
  variantId: '0e6f52df-72d8-44b2-96be-502aa0184816',
  name: 'The Heirloom Saree',
  category: 'sarees',
  label: 'Sarees',
  image: 'saree1.jpg',
  priceRupees: 8900,
};

function CartProbe() {
  const cart = useCart();
  return <>
    <output data-testid="summary">{cart.count}:{cart.subtotal}</output>
    <button type="button" onClick={() => void cart.add(product)}>Add</button>
    {cart.error && <p role="alert">{cart.error}</p>}
  </>;
}

describe('CartProvider', () => {
	afterEach(() => vi.unstubAllGlobals());
  it('loads and mutates a cart through credentialed API requests with CSRF', async () => {
    const snapshot = (count: number, subtotalPaise: number) => ({ data: {
      lines: count ? [{
        id: '648ba03a-415e-48b3-a247-7c79deab283e',
        variantId: product.variantId,
        productId: product.id,
        slug: product.slug,
        name: product.name,
        category: product.category,
        categoryName: product.label,
        imageKey: 'image assests/saree1.jpg',
        unitPricePaise: subtotalPaise,
        quantity: count,
      }] : [],
      count,
      subtotalPaise,
    } });
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(snapshot(0, 0)), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrfToken: 'signed-token' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(snapshot(1, 890000)), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    render(<CartProvider><CartProbe /></CartProvider>);

    await waitFor(() => expect(screen.getByTestId('summary').textContent).toBe('0:0'));
    fireEvent.click(screen.getByRole('button', { name: 'Add' }));
    await waitFor(() => expect(screen.getByTestId('summary').textContent).toBe('1:8900'));

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ credentials: 'include', method: 'GET' });
    expect(fetchMock.mock.calls[2][1]).toMatchObject({ credentials: 'include', method: 'POST' });
    expect(new Headers(fetchMock.mock.calls[2][1]?.headers).get('X-CSRF-Token')).toBe('signed-token');
    expect(JSON.parse(String(fetchMock.mock.calls[2][1]?.body))).toEqual({ variantId: product.variantId, quantity: 1 });
  });
});
