// Pembersih service worker legacy (kill-switch). JANGAN dihapus, JANGAN didaftarkan oleh v2.
//
// Aplikasi legacy di domain yang sama mendaftarkan service worker `/sw.js` (lingkup `/`, cache
// `mini-erp-pwa-v1-*`). v2 tidak memakai service worker, tetapi registrasi lama tetap hidup di
// perangkat (tablet kasir, browser kantor) dan browser memeriksa ulang `/sw.js` saat navigasi.
// Berkas ini adalah versi "baru" yang dipasang browser: ia menghapus cache, mencabut registrasinya
// sendiri, lalu memuat ulang tab supaya v2 berjalan tanpa service worker. Perangkat yang lama
// offline baru memeriksa ulang saat kembali dibuka, jadi berkas ini harus tetap disajikan.
//
// Sengaja tanpa handler `fetch`: tidak ada permintaan yang dicegat selama pembersihan.
// Disajikan server Go dengan `Cache-Control: no-store` (internal/server/server.go).

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      // v2 tidak memakai Cache Storage, jadi semua cache di origin ini milik legacy
      // (`mini-erp-pwa-v1-app`, `mini-erp-pwa-v1-runtime`, dan versi lain bila ada).
      const keys = await caches.keys();
      await Promise.all(keys.map((key) => caches.delete(key)));
      await self.registration.unregister();
      const windows = await self.clients.matchAll({ type: "window" });
      for (const client of windows) {
        // Gagal untuk tab yang tidak dikendalikan; tab itu memang sudah tanpa service worker.
        client.navigate(client.url).catch(() => undefined);
      }
    })(),
  );
});
