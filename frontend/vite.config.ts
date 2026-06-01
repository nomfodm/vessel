import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), wails("./bindings")],
  server: {
    // Bind both IPv4 and IPv6 so the Wails webview reaches Vite regardless of
    // how "localhost" resolves on the host (Windows prefers IPv6 ::1).
    host: true,
  },
});
