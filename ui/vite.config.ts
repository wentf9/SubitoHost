import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

export default defineConfig({
  plugins: [solid()],
  server: {
    proxy: {
      '/api/v1/stream': {
        target: 'ws://127.0.0.1:3301',
        ws: true,
      },
      '/api': {
        target: 'http://127.0.0.1:3301',
        changeOrigin: true,
      }
    }
  }
})
