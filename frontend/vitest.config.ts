/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    // Calendar tests read wall-clock times; pin the zone so CI (UTC) and laptops agree.
    env: { TZ: 'Asia/Ho_Chi_Minh' },
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    css: false,
  },
  resolve: {
    alias: {
      '@': '/src',
      'virtual:pwa-register': '/src/test/pwa-register-stub.ts',
    },
  },
})
