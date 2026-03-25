import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/auth': 'http://localhost:8080',
      '/problems': 'http://localhost:8080',
      '/games': 'http://localhost:8080',
      '/execute': 'http://localhost:8080',
    },
  },
})
