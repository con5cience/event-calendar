import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  root: "web",
  plugins: [react()],
  build: { outDir: "../dist", emptyOutDir: true },
  server: {
    proxy: {
      "/api": process.env.CALENDAR_API_ORIGIN || "http://127.0.0.1:8090",
      "/assets/favicon.png":
        process.env.CALENDAR_API_ORIGIN || "http://127.0.0.1:8090",
    },
  },
});
