import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5181,
    strictPort: true,
    // One origin for the browser: the backend sees Origin http://localhost:5181,
    // which must match its PUBLIC_URL for the CSRF check.
    proxy: {
      "/auth": { target: "http://127.0.0.1:8081", changeOrigin: false },
      "/devices": { target: "http://127.0.0.1:8081", changeOrigin: false },
    },
  },
  preview: { port: 5181, strictPort: true },
  test: { environment: "node", include: ["src/**/*.test.ts", "src/**/*.test.tsx"] },
});
