import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    // This repo lives on a Windows drive mounted into WSL2
    // (/mnt/d/...) — native filesystem watch events don't reliably cross
    // that boundary, so HMR silently stops picking up edits without
    // polling.
    watch: { usePolling: true, interval: 300 },
  },
})
