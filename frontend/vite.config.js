import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  base: process.env.DV_WEB_BASE || '/',
  plugins: [svelte()],
  clearScreen: false,
  server: {
    port: 34115,
    strictPort: true
  }
})
