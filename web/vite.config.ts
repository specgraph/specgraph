import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    proxy: {
      '/specgraph.v1': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
});
