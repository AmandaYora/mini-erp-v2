# Test Cases — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**
Semua ekspektasi dapat diverifikasi terhadap sistem lama sebelum sistem baru dianggap setara.

Sumber: `assistant.service.spec.ts` (intent 10 + rentang 5 + preview 12 + stats 4),
`whatsapp.service.spec.ts` (rate + otorisasi + DTO), `whatsapp-channel.service.spec.ts`,
`whatsapp-gateway.service.spec.ts` (mock-sesi!), `tools.service.spec.ts` (branch + query),
`knowledge.service.spec.ts` (create/detail/archive + `chunkText` + `inferDocumentType`),
E2E 19 (whitelist-UI) + E2E 10 (preview + stats) + E2E 01 (smoke), plus kasus turunan
pembacaan kode (ditandai **[dari kode]**). Envelope respons: `data.*` (E2E 10 membaca
`statsEnvelope.data?.stats`!). Ambig = **cek!**.

## Asisten & Tools (T-01–T-08)

- T-01 `Penjualan hari ini` → sales-hari-ini + 3-baris (bukan performa!). ✅ *ada di spec*
  (`preview` memanggil `getSalesSummary`; run-akhir `completed`).
  ```
  Request : { data: { message_text: "penjualan hari ini", mode: "rule_based" } }
  ```
  Diharapkan: `intent_type=="sales_summary"`, `answer_text` mulai
  `Penjualan hari ini untuk …:` + 3 baris IDR. **[dari kode]** urutan-menang:
  sales dicek sebelum performa!

- T-02 `ringkasan penjualan` → operasional (menang atas sales!). **[dari kode]**
  Diharapkan: `intent_type=="operational_summary"`, panggil `getOperationalSummary`
  (komposit!), jawaban `Ringkasan operasional …` 4-baris.

- T-03 `stok habis` → help (regex-sempit!); `Stok kritis apa saja?` → daftar-5. ✅ *ada di spec*
  Diharapkan: `stok habis` → `help` tanpa-tool; kritis → `getCriticalStock` +
  `Stok kritis untuk …` ≤5 baris (`slice(0,5)`!). E-09: 50-data → tampil-5-diam!

- T-04 `Info produk "X"` (nama!) → tak-ketemu (exact-code! KI-140). ✅ *ada di spec*
  (E2E 10 memakai pola ini dengan nama-produk-asli → ketemu karena LIKE-nama!
  **cek!** beda dengan klaim KI-140 — E2E lolos karena produk dicari via nama-LIKE,
  bukan kode!).
  ```
  Request : { data: { message_text: 'info produk "E2E-… Produk Pantauan"', mode: "rule_based" } }
  ```
  Diharapkan: `intent_type=="product_info"`, `answer_text` mengandung nama-produk.
  Lawan: `Info produk TanpaTandaPetikDenganNamaPanjang` → `LIKE %…%` bisa-ketemu;
  kode-salah-case ditangani (`toUpperCase` untuk exact!). Produk-arsip → null →
  `Saya belum menemukan produk yang cocok dengan kata kunci "…".`

- T-05 Tanpa-key → `bot_rule`-diam; 429-policy → kutipan-chunk-`bot_rule`. ✅ *ada di spec*
  (`delete OPENAI_API_KEY` → `resolution_mode=="bot_rule"` tanpa-suffix
  `Jika perlu…`; `generateRagAnswer` reject → `Berdasarkan dokumen internal` +
  `Retur barang` + mode `bot_rule`).

- T-06 Tool-DB-gagal → run-`failed` + 500-tanpa-jawaban. ✅ *ada di spec*
  (`getSalesSummary` reject `DB error` → `rejects.toThrow`, save-terakhir
  `status=="failed"` + `failureReason`; sama untuk `getCriticalStock` timeout!).

- T-07 Angka pending 50-data → tampil 5 (cap-diam! KI-139). **[dari kode]**
  Diharapkan: `GetPendingOrders(limit:5)` → jawaban ≤5 baris; komposit performa/
  operasional `pendingOrderCount==items.length` (≤5!) — bukan COUNT-SQL!

- T-08 Stats → 7-hari per intent/mode + rata-ms-bulat. ✅ *ada di spec*
  (`dataSource.query` dipanggil dengan `FROM assistant_runs` + `[companyId]`;
  baris `{day, intent_type:'sales_summary', resolution_mode:'bot_rule', total:'5',
  success:'4', avg_duration_ms:'123.45'}` → `{total:5, success:4, avg_duration_ms:123}`;
  `avg:null` → `null`; kosong → `{stats:[]}`).
  ```
  GET /assistant/runs/stats  Authorization: Bearer <localStorage mini-erp-access>
  ```
  Diharapkan: 200 + `data.stats` array (E2E 10!). Window SQL
  `DATE_SUB(CURDATE(), INTERVAL 7 DAY)`, urut `day DESC, total DESC`.

## WA (T-09–T-14)

- T-09 Connect → QR-8-detik → scan → connected + nomor-socket. **[dari kode]**
  Diharapkan: `POST whatsapp/channel/connect {}` → status (QR mungkin-null!);
  poll `POST whatsapp/status` (UI 3-detik!) hingga `qr_data_url` (data-URL!) atau
  `state=="connected"`; sesudah-scan: `state=="connected"`,
  `phone==bot_phone==digit(sock.user.id)`, `updated_at` baru. `display_number`
  input diabaikan!

- T-10 Duplikat-aktif → `Nomor {n} sudah terdaftar`; revoke → tambah-ulang-aktif. ✅ *ada di spec*
  Diharapkan: create-kedua-nomor-sama-aktif → **409**; `revoke` → tambah-ulang
  nomor-sama → sukses-aktif (reaktivasi-menimpa nama/akses!). Nomor-invalid
  (kosong/`<8`/`>15` digit, sesudah-buang-nondigit!) → **409**
  `Nomor WhatsApp tidak valid`.

- T-11 Update-revoked → tak-ketemu (buat-baru!); ganti-ke-revoked-orang → ditolak. **[dari kode]**
  Diharapkan: `update {id: revoked}` → **404** `Nomor WhatsApp tidak ditemukan`
  (lalu create-baru-nomor-sama sukses!); `update {id:A-aktif, phone: nomor-revoked-B}`
  → **409** `Nomor {n} sudah terdaftar`; `update {id:≤0/NaN}` → 404!

- T-12 Inbound-tak-dikenal → senyap (KI-137!); ke-11/menit → pesan-tunggu. ✅ *ada di spec* (rate!)
  Diharapkan **[dari kode]**: pesan-nyata nomor-tak-whitelist → gateway-telan-error
  (tanpa-balasan!); pesan ke-11 dalam 60-detik nomor-sama-company-sama →
  `sendText(remoteJid, "Terlalu banyak pesan dalam waktu singkat. Silakan tunggu sebentar.")`
  + `{run_id:null}`; grup/`@g.us`/`status@broadcast`/`fromMe`/stiker/audio →
  abaikan-diam; JID-LID-numerik lolos-digit (**cek!** collide!).

- T-13 Simulate-tanpa-connected → `Kanal WhatsApp belum terhubung` (nyata-bebas! KI-136). **[dari kode]**
  ```
  Request : { data: { phone_number: "6281…", message_text: "Penjualan hari ini" } }
  ```
  Diharapkan: channel `disconnected` → simulate **403**; pesan-nyata-sama (via gateway)
  tetap-diproses-tanpa-cek-kanal! Simulate sukses → reply `{run_id, branch_id,
  branch_name, intent_type, resolution_mode, answer_text}` + inbound/outbound
  tersimpan + tanpa-`sendText`!

- T-14 E2E-19: tambah → edit → hapus via UI tanpa-error. ✅ *ada di E2E*
  (`/assistant/setup`: `Tambah Nomor` + label `Nama pemilik nomor`/`Nomor pengirim WA`
  + `Simpan Nomor` → baris-terlihat-30s; `Edit` → `Simpan Perubahan` → nama-lama-count-0;
  `Hapus` → heading `Hapus Nomor Pengakses BOT` → `Hapus Nomor` → count-0-30s;
  nomor `6281{suffix}`/`6282{suffix}`; `expectNoRuntimeError`!).

## Knowledge (T-15–T-20)

- T-15 Tanpa-judul/file → 2-pesan-wajib. ✅ *ada di spec*
  Diharapkan: create tanpa-`file_name` → **400** `Nama file wajib diisi`;
  tanpa-`title` → **400** `Judul dokumen wajib diisi` (urutan file-dulu!).

- T-16 PDF-tanpa-teks → `ready` + skip-diam (KI-138!). **[dari kode]**
  Diharapkan: create `{file_name:"sop-x.pdf", file_base64: <pdf>, title, summary}`
  → 200 `status:"ready"` + versi `pending/pending`; `runPipeline` log-skip,
  status-tetap-`pending`, chunk-0; `detail` → `raw_text:null`,
  `chunking_status:"pending"`, `embedding_status:"pending"`!

- T-17 Update-tanpa-file → teks-timpa-kosong (KI-138!). **[dari kode]**
  Diharapkan: `update {id, title:"Baru"}` tanpa-`file_base64` →
  buffer-dari-fallback (judul/ringkasan!), versi+1, pipeline-ulang; bila tanpa
  ringkasan juga → `rawText` bisa-kosong → skip-diam! `currentVersionNo+1`,
  audit `knowledge.update {versionNo-1 → versionNo}`.

- T-18 Process → `{ queued:true }` langsung. **[dari kode]**
  Diharapkan: `POST knowledge/documents/process {id_knowledge_document}` →
  200 `{queued:true, version_no:<terbaru>}` seketika (pipeline-async!);
  tanpa-versi → 404 `Tidak ada versi dokumen ditemukan`; dok-tak-ada → 404
  `Dokumen tidak ditemukan`!

- T-19 Tanpa-dokumen → pesan-upload; chunk-kosong → pesan-indeksasi. ✅ *ada di spec*
  (mock `retrieveKnowledge {hasDocuments:false}` → jawaban-upload; policy
  `chunkCount:0` → jawaban-indeksasi!). **[dari kode]** AI-gagal → kutipan-3
  (`Berdasarkan dokumen internal:` + potong-400+`…`, mode-jatuh-`bot_rule`!).

- T-20 E2E-10: info-produk → nama-di-jawaban + intent-tepat. ✅ *ada di E2E*
  (lihat T-04; plus `GET stats` envelope + 4-path-tanpa-runtime-error!).

## Celah Test

- G-01 Tanpa test halaman assistant (QR/whitelist/upload!). E2E 19 hanya whitelist
  tambah/edit/hapus (tanpa connect-QR, tanpa revoked-duplikat, tanpa utama!).
- G-02 Tanpa test gateway-nyata (mock-semua!). `sendText`/reconnect/auth-reset/
  versi-socket/file-sesi belum-teruji (spec-gateway mock-socket!).
- G-03 Tanpa test batas-kuota/rate-mode-efektif. Kuota-50, NaN-lolos, rate-11,
  alias-mode-`ai`/`hybrid`, suffix-AI belum-teruji-E2E!
- G-04 Tanpa test JID-LID-numerik + tipe-pesan-sempit. Caption-gambar/video,
  stiker/audio/lokasi, `lid`-collide, thread-cabang-lama belum-teruji!
- G-05 Tanpa test pipeline-gagal-jujur. Skip-tanpa-teks, `skipped`-tanpa-key,
  `failed`-nol-embed, FULLTEXT-tak-dipakai, update-timpa-kosong belum-teruji-E2E!
