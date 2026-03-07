import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [tailwindcss()],
  base: '/',
  clearScreen: false,
  server: {
    host: true,
    strictPort: true,
    port: 5174,
    proxy: {
      '/api': {
        target: 'https://demo.forumline.net',
        changeOrigin: true,
        secure: true,
        configure: (proxy) => {
          // Disable buffering for SSE streams
          proxy.on('proxyRes', (proxyRes) => {
            if (proxyRes.headers['content-type']?.includes('text/event-stream')) {
              proxyRes.headers['cache-control'] = 'no-cache'
              proxyRes.headers['x-accel-buffering'] = 'no'
            }
          })
        },
      },
      '/auth': {
        target: 'https://demo.forumline.net',
        changeOrigin: true,
        secure: true,
      },
    },
  },
})
