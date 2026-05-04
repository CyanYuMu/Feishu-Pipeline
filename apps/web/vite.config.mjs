import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // TS backend (Fastify) - /api2 前缀映射到 /
      '/api2': {
        target: 'http://localhost:3001',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api2/, '/api'),
        onProxyReq: (proxyReq, req) => {
          console.log('[PROXY] /api2:', req.url, '->', proxyReq.path)
        },
        onError: (err, req, res) => {
          console.error('[PROXY ERROR]', err.message)
        }
      },
      // 飞书文档列表走 Go 后端（需要认证）
      '/api/feishu/documents': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/openapi': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/openapi/, '/public/openapi'),
      },
      '/api/projects': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/projects/, '/public/projects'),
      },
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
      },
    },
  },
  optimizeDeps: {
    include: ['@antv/x6', 'tslib'],
  },
})