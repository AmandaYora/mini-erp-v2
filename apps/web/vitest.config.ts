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
    // Tes halaman besar (userEvent + jsdom) menyentuh ~5 dtk saat ratusan berkas
    // berjalan paralel di mesin yang sibuk; tes yang sama lulus sendirian. Batas
    // 5 dtk bawaan membuat gerbang flaky, bukan menangkap bug — longgarkan.
    testTimeout: 20_000,
    hookTimeout: 20_000,
  },
});
