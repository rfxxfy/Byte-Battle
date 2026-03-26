import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        ws: true,
        bypass: (req) => {
          if (req.headers['accept']?.includes('text/html') || req.headers['sec-fetch-mode'] === 'navigate') return '/index.html'
        },
      },
    },
  },
})
