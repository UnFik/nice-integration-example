import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const backend = { target: "http://127.0.0.1:8083", changeOrigin: false };

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5182,
    strictPort: true,
    // One origin for the browser: the backend sees Origin http://localhost:5182,
    // which must match its PUBLIC_URL for the CSRF check.
    proxy: {
      "/auth": backend,
      "/chat": backend,
      "/mcp": backend,
      "/.well-known": backend,
    },
  },
  preview: { port: 5182, strictPort: true },
  test: { environment: "node", include: ["src/**/*.test.ts", "src/**/*.test.tsx"] },
});
