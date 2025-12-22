/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path"

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:9096",
        changeOrigin: true,
        secure: false,
        rewrite: (path) => '/api' + path.replace(/^\/api/, ''),
      }
    }
  },
  build: {
    outDir: "dist",
    emptyOutDir: true
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    exclude: ["node_modules", "dist", "tests/*", "**/tests/**"]
  }
});
