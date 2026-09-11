# User Flows — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A.** Aktor: Owner/Admin (`whatsapp.manage` + `knowledge.*`),
Viewer (`whatsapp.view`/`knowledge.view`), Superadmin (`whatsapp.simulate` tambahan,
migrasi 028!), Pengirim-WA (whitelist). Semua request POST `{data}` kecuali stats
GET. Scope perusahaan dari sesi. Ambig = **cek!**.

---

## F-01 Hidupkan BOT (setup sekali!)

1. `/assistant/setup` → guard cabang/workspace (tanpa → blank!) → slice reload 4
   paralel (`documents/list`, `authorizations/list`, `status`, `config/get`).
2. Panel Nomor BOT `disconnected`: klik `Hubungkan Nomor BOT (QR)` → `requestQr()`
   (`setShowQr(true)` + `POST whatsapp/channel/connect {}`) → server
   reset-storage + boot-Baileys + tunggu-8-detik → return status (QR mungkin-belum-ada!).
3. Blok QR tampil (`showQr`): polling-3-detik `POST whatsapp/status` hingga
   `connected`; bila `qr_data_url` ada → `<img 220px>` + `Scan QR Nomor BOT` +
   `Buka WhatsApp perusahaan > Perangkat Tertaut > Tautkan Perangkat.`; bila
   `last_error_text` tanpa-QR → `QR Belum Bisa Dibuat` + `QR gagal dibuat` +
   `WhatsApp menolak sesi pairing sebelum QR diterbitkan…` + `Muat Ulang QR BOT`.
4. Scan QR dengan WA perusahaan (sebagai linked-device!) → event `open` → nomor
   dari `sock.user.id` → `connected`: QR auto-tutup, panel `Terhubung` + nomor +
   `Diperbarui` + `Putuskan Koneksi`. QR-gagal → `Muat Ulang QR BOT` (connect-ulang
   = reset!); `reconnecting` tanpa-QR → `Tampilkan QR BOT`/`Reset Status`
   (=disconnect!).
5. `Tambah Nomor` → modal (nama + nomor + akses) → validasi-front 8–15-digit →
   `POST whatsapp/authorizations/create` → toast `Nomor pengakses BOT tersimpan`
   → reload → baris-tabel (badge `Terotorisasi/Owner`, `Aktif`). Ulangi per
   pengirim. Tanpa-baris = semua pesan ditolak (403→senyap-nyata!). Edit via
   `…/update`, hapus via `…/revoke` (toast `…diperbarui`/`…dihapus`).
6. Gagal khas: tanpa-izin-kelola → tanpa-tombol (viewer!); QR-timeout-8-detik →
   polling-terus; disconnect = `POST whatsapp/channel/disconnect` → toast
   `Nomor BOT diputuskan` → wajib-scan-ulang!

## F-02 Atur Cara Menjawab + SOP

1. `/assistant/config` → `POST whatsapp/assistant-config/get` (via reload!) →
   select `Mode jawaban` (`BOT Standar`/`BOT Dibantu AI`) → `onChange` langsung
   `POST whatsapp/assistant-config/update {mode}` → toast `Mode asisten diperbarui`
   → reload. Viewer: teks `Mode aktif` + warning `Anda hanya punya akses lihat…`.
   Notice-info selalu tampil (deskripsi-mode). Kuota/sisa tak-tampil!
2. Butuh `knowledge.view` else kartu-warning `Akun Anda belum memiliki izin…`.
   `Upload` (butuh `knowledge.create`) → modal (`Judul dokumen*` + `Tipe dokumen`
   + `Nama file*` + `File dokumen` (pilih-file → auto-nama + base64!) +
   `Ringkasan isi*`) → `POST knowledge/documents/create` → toast
   `Dokumen knowledge berhasil disimpan` → reload → badge `Siap` (langsung-`ready`,
   walau pipeline-belum-jalan!). Klik-baris → detail-kanan (KV + notice-indeks
   bila `processing` / gagal bila `failed`); `Arsipkan Dokumen` (butuh
   `knowledge.archive`) → `POST …/archive` → toast `…diarsipkan`.
3. Update-dokumen & process & detail-mentah tanpa-UI (API saja!): update via
   `POST knowledge/documents/update` (versi+1 + pipeline-ulang); antre-ulang via
   `POST knowledge/documents/process` → `{queued:true}`.
4. Cek kapabilitas + keterbatasan (statis — janji yang harus ditepati backend!):
   4 kartu contoh + 2 kartu batas (`Tidak bisa ubah data`, `SOP butuh dokumen siap`).

## F-03 Bertanya via WA (harian pengirim!)

1. Pengirim-whitelist kirim pesan ke nomor-BOT (mis. `Penjualan hari ini`,
   `/help`, `Stok kritis`, `Info produk "X"`, `Apa kebijakan retur?`).
2. Gateway (`notify`, bukan-milik-sendiri, bukan-grup/broadcast) ekstrak-teks →
   digit-JID → `processInboundMessage(sendViaGateway=true, idBranch=null)`:
   digit-valid? auth-aktif? (revoked/tak-dikenal → 403-ditelan = senyap!) rate-10/menit?
   (ke-11 → balasan-tunggu-via-gateway!) → mode-efektif (AI bila key+kuota!)
   → thread-open → simpan-inbound → `assistant.execute` (cabang = substring-nama
   di-pesan, else seluruh-perusahaan!) → simpan-outbound → `sendText` ke `remoteJid`.
3. Terima jawaban format-tetap (9 intent!): sales 3-baris / pending ≤5 /
   status-heading / kritis ≤5 / performa-hari-ini / produk / operasional / SOP
   (siap → kutipan/AI; kosong → pesan-upload; indeks → pesan-tunggu) / help.
4. Tak-dikenal → menu help; tak-terotorisasi → senyap (KI-137!); grup/stiker/audio/
   lokasi → abaikan-diam; AI-gagal/timeout/429 → kutipan-chunk (policy saja!);
   tool-DB-gagal → inbound-`failed`, tanpa-balasan (nyata-ditelan!).
5. Cabang: sebut nama/kode/kota cabang di pesan untuk data-cabang (else
   seluruh-perusahaan!); preview/simulate bisa paksa `id_branch`.

## F-04 Uji & Pantau (teknis!)

1. Simulate via API (`POST whatsapp/messages/simulate`, `whatsapp.manage` —
   tanpa-UI!): `{phone_number, message_text, provider_message_id?, id_branch?}`
   → syarat **connected** (nyata-bebas!) → tanpa-rate/filter-JID → tanpa-kirim →
   error-diteruskan (403/500!). Beda-7-vs-nyata (KI-136!).
2. Preview via API (`POST assistant/preview`, superadmin `whatsapp.simulate`!):
   `{message_text, id_branch? (=sesi-bila-kosong!), mode?}` → jawaban + run-id +
   intent + mode (tanpa-inbox/thread!; tanpa-cek-kuota!).
3. Stats via GET (`assistant/runs/stats`, `whatsapp.view`): 7-hari per
   intent/mode (`total/success/avg_duration_ms-bulat`). Tanpa-filter/pagination/UI!
4. E2E 19: `/assistant/setup` tambah (`Tambah Nomor`→`Simpan Nomor`) → edit
   (`Edit`→`Simpan Perubahan`, nama-lama-hilang!) → hapus (`Hapus`→heading
   `Hapus Nomor Pengakses BOT`→`Hapus Nomor`, baris-hilang!) via UI tanpa-error
   (timeout 30 detik!).
5. E2E 10: `POST assistant/preview {message_text:'info produk "{nama}"',
   mode:'rule_based'}` → jawaban mengandung nama + `intent_type=='product_info'`;
   lalu `GET /assistant/runs/stats` (Bearer `localStorage mini-erp-access`) →
   envelope `data.stats` array; lalu 4 path (dashboard/audit-logs/setup/config)
   tanpa-runtime-error. E2E 01: smoke `/assistant/setup|config` cocok-teks.
6. Unit-spec: assistant (intent+rentang+preview+stats!), whatsapp (rate+otorisasi!),
   tools (branch+query!), knowledge (create/detail/archive+chunk+tipe!) — lihat test-cases.
