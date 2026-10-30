import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { defineConfig as defineVitestConfig } from 'vitest/config';
import { VitePWA } from 'vite-plugin-pwa';

export default defineVitestConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'prompt',
      includeAssets: ['pwa/aramya-mark.svg'],
      manifest: {
        id: '/',
        name: 'Aramya Fashion House',
        short_name: 'Aramya',
        description: 'Considered occasionwear, bridal couture and made-to-measure style by Shubha Bharat.',
        theme_color: '#8c1721',
        background_color: '#f7f4ef',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        icons: [
          { src: '/pwa/aramya-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/pwa/aramya-512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' },
          { src: '/pwa/aramya-mark.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,webp,jpg,jpeg,avif,woff2}'],
        navigateFallback: 'index.html',
        navigateFallbackDenylist: [/^\/api\//, /^\/(?:checkout|account|admin)(?:\/|$)/],
        runtimeCaching: [
          {
            urlPattern: ({ url, request }) => request.method === 'GET' && url.origin === self.location.origin && /^\/api\/v1\/(?:categories|products(?:\/[^/]+)?)$/.test(url.pathname),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'aramya-public-catalog-v1',
              networkTimeoutSeconds: 4,
              fetchOptions: { credentials: 'omit' },
              expiration: { maxEntries: 80, maxAgeSeconds: 60 * 60 * 24 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
          {
            urlPattern: ({ url, request }) => request.method === 'GET' && url.origin === self.location.origin && /^\/media\/[^/]+\.(?:avif|jpe?g|png|webp)$/i.test(url.pathname),
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'aramya-public-images-v1',
              expiration: { maxEntries: 100, maxAgeSeconds: 60 * 60 * 24 * 30 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
      devOptions: { enabled: false },
    }),
  ],
  test: { environment: 'jsdom' },
  server: {
    fs: { allow: ['..'] },
    proxy: {
      '/api': { target: 'http://127.0.0.1:18080', changeOrigin: true },
    },
  },
});
