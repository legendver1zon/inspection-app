import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const backend = 'http://localhost:8090'

// Dev-прокси. Помимо /api и статики, в бэкенд ходят легаси-маршруты:
// скачивание фото/документов и сохранение формы (POST /inspections/:id/edit).
// GET /inspections/* — страницы SPA, их отдаёт Vite (index.html).
export default defineConfig({
  plugins: [react(), tailwindcss()],
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
    },
  },
})
