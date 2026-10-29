import { describe, expect, it } from 'vitest';
import { categories, formatPrice, products } from './catalog';

describe('source catalog', () => {
  it('preserves all 39 original products and their five collection totals', () => {
    expect(products).toHaveLength(39);
    expect(categories.map(({ id }) => products.filter((product) => product.category === id).length)).toEqual([5, 5, 14, 10, 5]);
    expect(new Set(products.map(({ name }) => name)).size).toBe(39);
  });

  it('formats source prices in Indian rupees', () => {
    expect(formatPrice(8900)).toBe('₹ 8,900');
  });
});
