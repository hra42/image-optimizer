import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

// `base: './'` (below) rewrites every in-HTML URL to a relative path so the
// embedded SPA's assets resolve at any mount path. That's right for
// favicons/CSS/JS, but WRONG for Open Graph / Twitter image tags: social
// crawlers resolve og:image against the page origin and need an absolute or
// root-relative URL, not `./og.svg`. This plugin restores the root-relative
// form for those meta tags after Vite's rewrite.
function absoluteSocialMeta() {
  return {
    name: 'absolute-social-meta',
    transformIndexHtml(html) {
      return html.replace(
        /(<meta\s+(?:property|name)="(?:og:image|twitter:image)"\s+content=")\.\/([^"]+)(")/g,
        '$1/$2$3',
      );
    },
  };
}

// Build output goes to dist/, which Go embeds via //go:embed.
// base: './' makes asset URLs relative so they resolve no matter what path the
// embedded SPA is served from by Fiber.
export default defineConfig({
  base: './',
  plugins: [svelte(), tailwindcss(), absoluteSocialMeta()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
  },
});
