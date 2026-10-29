import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { HomePage } from './HomePage';

describe('home page', () => {
  it('shows the Aramya hero and all five original collections', () => {
    render(<MemoryRouter><HomePage /></MemoryRouter>);

    expect(screen.getByRole('heading', { name: /wear the moment/i })).toBeTruthy();
    expect(screen.getByRole('heading', { name: /find your occasion/i })).toBeTruthy();
    expect(screen.getByRole('link', { name: /sarees a drape for every celebration/i })).toBeTruthy();
    expect(screen.getByRole('link', { name: /bridal & lehengas for the day you will always remember/i })).toBeTruthy();
    expect(screen.getByRole('link', { name: /occasionwear festive sets, gowns and shararas/i })).toBeTruthy();
    expect(screen.getByRole('link', { name: /menswear considered festive dressing/i })).toBeTruthy();
    expect(screen.getByRole('link', { name: /little ones a little something for their big day/i })).toBeTruthy();
  });
});
