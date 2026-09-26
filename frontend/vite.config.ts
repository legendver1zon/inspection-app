import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

const backend = 'http://localhost:8090'

// Dev-прокси. Помимо /api и статики, в бэкенд ходят легаси-маршруты:
// скачивание фото/документов и сохранение формы (POST /inspections/:id/edit).
// GET /inspections/* — страницы SPA, их отдаёт Vite (index.html).
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // Офлайн: оболочка приложения и последние ответы API кешируются
    // Service Worker'ом, чтобы акт открывался и без сети.
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'script-defer',
      includeAssets: ['icons/*.png'],
      manifest: {
        name: 'АктОсмотр',
        short_name: 'АктОсмотр',
        lang: 'ru',
        start_url: '/inspections',
        scope: '/',
        display: 'standalone',
        background_color: '#F8F6F1',
        theme_color: '#3E68A8',
        icons: [
          { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icons/icon-512-maskable.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        clientsClaim: true,
        skipWaiting: true,
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/inspections\/new$/, /^\/api\//, /^\/photos\//, /^\/documents\//, /^\/static\//, /^\/defects\//, /^\/healthz/, /^\/assets\//, /\.[a-z0-9]+$/i],
        globPatterns: ['**/*.{js,css,html,png,svg,woff2}'],
        maximumFileSizeToCacheInBytes: 4 * 1024 * 1024,
        cleanupOutdatedCaches: true,
        runtimeCaching: [
          {
            urlPattern: ({ url, request }) => request.method === 'GET' && url.pathname.startsWith('/api/'),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api',
              networkTimeoutSeconds: 8,
              expiration: { maxEntries: 300, maxAgeSeconds: 14 * 24 * 3600 },
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            urlPattern: ({ url }) => /^\/photos\/\d+\/thumb$/.test(url.pathname),
            handler: 'CacheFirst',
            options: {
              cacheName: 'thumbs',
              expiration: { maxEntries: 1000, maxAgeSeconds: 60 * 24 * 3600 },
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/static/'),
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'static', expiration: { maxEntries: 200 } },
          },
        ],
      },
    }),
  ],
  server: {
    proxy: {
      '/api': backend,
      '/static': backend,
      '/photos': backend,
      '/documents': backend,
      '/defects': backend,
      '/inspections': {
        target: backend,
        bypass: (req) => {
          if (req.method === 'GET' || req.method === 'HEAD') return '/index.html'
          return undefined
        },
      },
      '/profile': {
        target: backend,
        bypass: (req) => {
          if (req.method === 'GET' || req.method === 'HEAD') return '/index.html'
          return undefined
        },
      },
    },
  },
})
