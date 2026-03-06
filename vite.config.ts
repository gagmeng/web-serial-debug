import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [vue()],
  // GitHub Pages needs the repo base path, while the desktop app must use
  // relative asset URLs so file:// loading works on Windows/macOS/Linux.
  base: mode === 'desktop' ? './' : '/web-serial-debug/',
  build: {
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor': [],
          'vue': ['vue', 'vue-router', '@vueuse/core', 'pinia'],
          'three': ['three', 'stats.js', 'three-particle-fire'],
          'uplot': ['uplot'],
          'xterm': ['xterm', 'xterm-addon-fit', 'xterm-addon-web-links', '@xterm/addon-search'],
          'utils': ['splitpanes', 'element-plus']
        }
      }
    }
  },
  define: {
    __BUILD_TIME__: JSON.stringify(new Date().toISOString())
  }
}))
