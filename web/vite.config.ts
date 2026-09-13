import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The build lands inside the Go module so `go build` embeds it.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/server/static/dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://127.0.0.1:51821", changeOrigin: false },
      "/metrics": { target: "http://127.0.0.1:51821", changeOrigin: false },
    },
  },
});
