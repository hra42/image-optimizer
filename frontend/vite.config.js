import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

// Build output goes to dist/, which Go embeds via //go:embed.
// base: './' makes asset URLs relative so they resolve no matter what path the
// embedded SPA is served from by Fiber.
export default defineConfig({
  base: './',
  plugins: [svelte(), tailwindcss()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
  },
});
