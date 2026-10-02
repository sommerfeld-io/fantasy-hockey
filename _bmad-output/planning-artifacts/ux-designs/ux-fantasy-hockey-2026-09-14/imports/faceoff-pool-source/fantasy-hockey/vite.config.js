import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  // Relative asset paths so the build works served from any path
  // (domain root, a /subpath/, GitLab Pages, opened from file://, etc.)
  base: "./",
  plugins: [react(), tailwindcss()],
});
