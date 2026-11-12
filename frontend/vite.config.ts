import { fileURLToPath, URL } from 'node:url';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Vite 配置：@ 别名指向 src，便于深层引用
export default defineConfig({
  // Wails 桌面版：资源以相对路径加载（生产包从 frontend/dist 内嵌到 exe）
  base: './',
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // wails dev 模式下 WebView2 以 wails.localhost:随机端口 访问 dev server
    allowedHosts: true,
  },
});
