import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { defineConfig as defineVitestConfig } from 'vitest/config';

export default defineVitestConfig({
  plugins: [react()],
  test: { environment: 'jsdom' },
  server: {
    fs: { allow: ['..'] },
  },
  publicDir: '../image assests',
});
