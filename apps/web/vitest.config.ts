import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    // `react/index.js` memilih build dari process.env.NODE_ENV, dan `act`
    // HANYA ada di react.development.js. Tanpa penguncian ini, menjalankan
    // test dengan NODE_ENV=production (lazim di CI dan Docker build) membuat
    // setiap test yang me-render komponen gagal dengan
    // "React.act is not a function" — bukan karena kodenya salah.
    env: { NODE_ENV: "development" },
  },
});
