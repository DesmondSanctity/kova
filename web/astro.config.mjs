// @ts-check
import { defineConfig } from 'astro/config';

import svelte from '@astrojs/svelte';
import tailwindcss from '@tailwindcss/vite';

const proxy = Object.fromEntries(
 ['/api', '/v1', '/auth', '/health', '/r/', '/v/', '/pay', '/assets'].map(
  (p) => [p, { target: 'http://localhost:8080', changeOrigin: true }],
 ),
);

// https://astro.build/config
export default defineConfig({
 site: 'https://usekova.pages.dev',
 integrations: [svelte()],

 vite: {
  plugins: [tailwindcss()],
  server: { proxy },
 },
});
