import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // Same shape as production nginx: /api/* goes to the Go API with the prefix
    // stripped, so the page and API share an origin and CORS never applies,
    // whichever port Vite ends up on.
    proxy: {
      "/api": { target: "http://localhost:8080", rewrite: (path) => path.replace(/^\/api/, "") },
    },
  },
});
