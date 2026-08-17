import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // This repo lives on a Windows drive mounted into WSL2
    // (/mnt/d/...) — native filesystem watch events don't reliably cross
    // that boundary, so HMR silently stops picking up edits without
    // polling.
    watch: { usePolling: true, interval: 300 },
  },
})
