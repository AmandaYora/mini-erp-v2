# User Flows — Modul 18 Dashboard

**Kelompok A.** Aktor: semua role (staff pun boleh!) dengan cabang aktif.

---

## F-01 Pantau Pagi Hari (alur utama)

1. Login → pilih cabang → `/dashboard` terbuka default.
2. Baca 4 KPI: perlu-aksi (klik → daftar pending), net-bulan (klik → selesai),
   piutang (klik → finance), kritis (klik → stok kritis).
3. Baca strip: net-hari + hitung-pesanan, utang, produk-dipantau, WA/SOP.
4. Baca tren + 2 ranking + prioritas + kritis; klik baris → detail
   (produk/order/stok).
5. Ganti cabang → semua angka refetch otomatis.

## F-02 Kejar Keterlambatan (prioritas)

1. Kartu `Order Perlu Aksi` / seksi Pesanan Prioritas → alasan
   (`Lewat jatuh tempo` → dahulukan!).
2. Klik baris → `/orders/{id}` → tindaklanjuti di modul order.
3. Kosong → `Tidak ada pesanan prioritas` (keadaan sukses, bukan kosong-error!).

## F-03 Cek Margin Sebelum Belanja Stok

1. Seksi Estimasi Margin → produk margin-tertinggi + HPP-estimasi + persen.
2. Bila warning cost-tak-lengkap → lengkapi harga-beli/modal di produk/saldo-awal
   (modul 05/17h!) lalu kembali — angka membaik.
3. Ingat: estimasi operasional, final di Finance pasca-posting!

## F-04 Verifikasi API (pola E2E 10)

1. `POST dashboard/summary` (`today_from/to` ISO-UTC `2026-05-10T00:00:00.000Z` s/d
   `23:59:59.999Z` + cabang via sesi BLR; `10-observability…:46–85`).
2. `today_order_count ≥ 1`, `today_sales_amount ≥ 20000` (net `money()` 2-desimal!),
   `tracked_product_count > 0`.
3. Tanpa token → 401 (`152–157`, body `{data:{}}`). Buka `/dashboard` + 3 halaman pantauan
   (`/audit-logs`, `/assistant/setup`, `/assistant/config`) → sidebar tampil +
   tanpa runtime-error (137–141).

## F-05 Gagal Diam-Diam → Fallback (tanpa test, verifikasi kode!)

1. `apiPost` gagal (`catch` 228–231) → `setDashboardSummary(null)`.
2. Halaman tetap render penuh dari `activeWorkspaceData` lokal: hitung-hari, omzet-hari
   tanpa-retur, pending/active, kritis per-baris-lokasi, prioritas tanpa-skor (terbaru dulu,
   maks 5, alasan 2-teks!), tren 6-bulan tanpa-retur, WA dari `whatsappChannelStatus`,
   SOP `ready ?? total` (238–313, 315–353!). Tanpa penanda (KI-132!).
3. Tanpa cabang/perusahaan di store → render `null` (bukan fallback, bukan error!).
