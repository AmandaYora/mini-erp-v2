# Business Rules — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A.** Sumber: baca penuh `assistant.service.ts` (656),
`tools.service.ts` (340), `whatsapp.service.ts` (563),
`whatsapp-gateway.service.ts` (462), `whatsapp-channel.service.ts` (65),
`knowledge.service.ts` (552), 3 controller, 9 entity, migrasi 003/004/028,
web `assistant/` + `assistant.slice.ts`. Klaim tanpa jejak kode ditandai **cek!**.

---

## 1. Akses (18 endpoint!)

| ID | Aturan |
|---|---|
| BR-01 | `POST assistant/preview` = `whatsapp.simulate` (superadmin-only; migrasi 028 memberi ke role id 1!; satu-satunya pemakai izin ini); `GET assistant/runs/stats` = `whatsapp.view` (satu-satunya GET!). Keduanya di balik `JwtAuthGuard + PermissionGuard`. |
| BR-02 | Kanal/write (`connect/disconnect/authorizations-create/update/revoke/simulate/config-update`) = `whatsapp.manage`; baca (`status/list/get`) = `whatsapp.view`. Semua 10 endpoint whatsapp POST `@HttpCode(200)`. |
| BR-03 | Dokumen: lihat (`list/detail`) = `knowledge.view`; buat = `knowledge.create`; ubah + proses (`update/process`) = `knowledge.update`; arsip = `knowledge.archive`. Semua 6 POST `@HttpCode(200)`. |
| BR-04 | Staff tanpa keduanya (default role-access-config!); halaman `/assistant/*` butuh `whatsapp.view` kecuali 3 alias knowledge yang butuh `knowledge.view`; halaman tetap butuh cabang + workspace aktif (else blank-null!). |
| BR-04b | Scope perusahaan/cabang/user selalu dari sesi (`ActiveSessionPayload`), bukan body. `id_branch` body hanya override opsional (preview/simulate); otorisasi/config/list selalu `session.idCompany`. |

## 2. Asisten & Tools (21a)

| ID | Aturan |
|---|---|
| BR-05 | Mode request: preview `normalizeRequestedMode` — `ai_assisted`-persis else `rule_based` (alias `ai`/`hybrid` via preview = rule!). Mode eksekusi preview: `resolveExecutableMode` — `ai_assisted` + `OPENAI_API_KEY` non-kosong else `rule_based` (tanpa cek kuota!). Mode inbound: `resolveEffectiveMode` — `ai_assisted` + key + `COUNT(bot_ai hari-ini) < ai_daily_limit (default 50)` else `rule_based`. Suffix-AI hanya bila mode akhir `ai_assisted`. AI hanya dipakai jalur `policy_qna` (`generateRagAnswer`); 8 intent lain selalu rule meski mode AI! |
| BR-05b | `resolution_mode` tersimpan = `bot_ai` bila mode-akhir AI else `bot_rule`; ditulis 2× (saat create-run + saat complete). `llm_model_name` selalu `null` (model `gpt-4o-mini`/env tak-pernah-disimpan!). |
| BR-06 | Intent urutan-menang (cek `detectIntent`, `assistant.service.ts:365-382`): help-eksplisit (`/help\|help\|bantuan\|menu\|fitur` persis ATAU `apa saja.*(fitur\|bisa)`/`cara pakai`) → policy (`sop\|kebijakan\|policy\|aturan`) → operasional (`ringkasan\|summary\|kondisi bisnis\|operasional`) → sales (`penjualan\|sales\|omzet\|revenue`) → pending (`pending\|menunggu\|belum diproses\|belum selesai`) → status (wajib kata order/pesanan/transaksi + kata-status) → kritis (`stok kritis\|stok menipis\|restock\|stok tipis`) → performa (`performa\|performance\|hari ini`) → produk (`produk\|barang\|item`) → help. Akibat: `stok habis` → help!; `Info produk X hari ini` → performa (sebelum produk!)!; `ringkasan penjualan` → operasional!; `Penjualan hari ini` → sales BUKAN performa! |
| BR-06b | Status-filter (`resolveOrderStatusFilters`): draft → `(pending,draft)`; dikonfirmasi/confirmed/konfirmasi → `(active,confirmed)`; diproses/in progress/in_progress/proses → `(active,in_progress)` — cek! `proses` umum bisa menelan kata lain; selesai/completed/complete → `(completed,completed)`; dibatalkan/batal/cancelled/canceled → `(cancelled,cancelled)` — cek! `batal` menelan `dibatalkan` wajar; aktif/active → `(active, undefined)`; else `(undefined, undefined)` + heading `Order terbaru` + empty `''` → fallback `Belum ada order {label}…`. |
| BR-06c | Rentang (`resolveDateRange`): `kemarin` → H-1 00:00–23:59 label `kemarin`; `minggu ini`/`7 hari` → H-6 00:00 s.d. hari-ini 23:59 label `7 hari terakhir`; `bulan ini` → tgl-1 00:00 s.d. kini label `bulan ini`; else hari-ini 00:00–23:59 label `hari ini`. Hanya dipakai sales/order-status/operasional; performa selalu hari-ini! |
| BR-06d | Produk-query (`extractProductQuery`): `"..."` pertama else buang kata `info/detail/status/stok/produk/barang/item` + rapikan-spasi; kosong → query-mentah. Pemanggil upper-case-kan untuk `productCode` exact, nama-asli untuk LIKE. |
| BR-07 | Run: `running` → `completed` (+jawaban render-mode + `completedAt`) / `failed` (+`failureReason=(error).message` + `completedAt` + rethrow!). Tool 1-per-intent (help-nol-tool-tanpa-baris!; policy selalu `RetrieveKnowledge` meski tanpa-dokumen!). Tiap tool tulis 1 baris (`sequence_no` mulai 1, `success`/`failed` — tanpa `skipped`!; `input_json`/`output_json`/`duration_ms=Date.now-started`/`error_text`/`executed_at`). Tool-gagal → run `failed` + HTTP-500 tanpa jawaban ramah! |
| BR-07b | Format jawaban: sales 3-baris; pending `Order pending…`/`Tidak ada…` (maks 5!); status `{heading} {label}…`/`{empty} {label}…`/`Belum ada order…`; kritis `Stok kritis…`/`Tidak ada…` (`slice(0,5)`!); performa `Snapshot performa hari ini…` 4-baris (selalu hari-ini!); produk 7-baris/`Saya belum menemukan…`; operasional 4-baris; policy-a/b/c; help 14-baris; mata-uang `Intl id-ID IDR 0-desimal`. |
| BR-08 | Angka pending/kritis = panjang-array-limit-5 (KI-139!); produk = exact-code-dulu (KI-140!); cabang-asing = seluruh-perusahaan (`resolveBranchContext` gagal → `{null,null}` tanpa-error!). Limit SQL `min(input??5,20)`; kritis tanpa-limit-SQL (cap di format!). |
| BR-09 | Tanpa validasi DTO preview (terima semua, `?? ''`/`?? null`!); kuota AI 50/hari (`assistant_preferences.ai_daily_limit`, `Number()` — NaN lolos!); rate-WA 10/menit in-memory (hilang-restart!); timeout AI 15 detik (`AbortController`); embedding dipotong 8000 char. |

## 3. WhatsApp (21b)

| ID | Aturan |
|---|---|
| BR-10 | Satu sesi per perusahaan (`Map` + unik `id_company`!); connect = `ensureChannel` + `startPairing` bila belum-connected (reset: tutup-socket + hapus-sesi + `rm -rf`+`mkdir company-{id}` + `disconnected` → boot → tunggu-QR-8-detik-poll-250ms, timeout-diam!); QR = `toDataURL` in-memory (hilang-restart!); nomor-BOT-dari-`sock.user.id` (input-`display_number`-abaikan!; JID-kosong = nomor-lama-bertahan!); boot-default `qrTimeout 60s/connect 30s/query 60s`, browser Ubuntu-Chrome, silent-logger; reconnect-kecuali-hitam (515-selalu; pernah-connected + bukan {401,403,405,419,411,440,500}, tunda 1500 ms); reset-auth {401,403,405,419,411}+500; disconnect = logout + hapus-storage (wajib-scan-ulang!). Storage default `storage/whatsapp-sessions/` (env `WHATSAPP_SESSION_DIR`!), versi `WHATSAPP_BAILEYS_VERSION`/`WHATSAPP_USE_PACKAGE_DEFAULT_VERSION`. |
| BR-11 | Nomor 8–15-digit (`^\d{8,15}$` sesudah buang-nondigit!); akses 2-level + utama-tunggal (auto-bila-pertama-owner! + demote-semua-lain!; update mempertahankan-utama-lama atau auto-bila-tak-ada-utama-lain!); duplikat-aktif-tolak (`Nomor {n} sudah terdaftar`); revoke-status + lepas-utama (bukan-hapus!); reaktivasi-menimpa (nama/akses/utama/`lastSeenAt=null`); update-aktif-saja (revoked = 404-buats-baru!); daftar default-aktif (+`include_revoked`!); unik DB `(id_company, phone_e164)`; DTO gandakan `phone`/`sender_phone`! |
| BR-12 | Inbound: filter (`notify`+bukan-milik-sendiri+bukan-grup/broadcast+teks-non-kosong!) + digit-valid + aktif-wajib (SEBELUM-run!) + syarat-kanal (simulate-wajib-connected, nyata-bebas!) + rate-hanya-nyata + mode + thread-open-per-(company,kanal,otorisasi,nomor) + `lastMessageAt/lastSeenAt=now` + simpan-inbound-`received`→`processed`/`failed` + outbound-`processed`(+runId) + kirim-gateway-hanya-nyata; gagal-nyata-ditelan, gagal-simulate-diteruskan. Simulate 7-beda (KI-136!): filter-JID, cabang-bisa-diisi (nyata-null!), syarat-kanal, rate, kirim, error, rawPayload (`{simulated:true}` vs `{provider:'baileys',key}`)! |
| BR-13 | Config: `ensureSettings` buat-baris-kosong!; alias-AI (`ai`/`hybrid`/`ai_assisted`) diterima, asing → rule-diam; tulis-merge (`whatsapp_mode` + `behavior_preferences`); baca `whatsapp_mode ?? mode ?? 'rule_based'`; efektif-tak-tampil + kuota-tak-tampil (KI-141!). UI simpan `{mode}` saja (behavior tak-ada-UI!). |

## 4. Knowledge (21c)

| ID | Aturan |
|---|---|
| BR-14 | Judul + nama-file-wajib (400!); tipe-eksplisit-menang else infer-nama (`sop`→sop, `policy`/`kebijakan`→policy, `glos`→glossary, else guide); buffer base64 else fallback `title+summary/description+raw_text`; `rawText` hanya txt/md/csv/json else fallback-trim/null; `sha256(buffer)` (bukan-teks!); transaksi dokumen+versi; file-private (`visibility:'private'`, key `buildKnowledgeSourceKey`); versi-tiap-ubah (`currentVersionNo+1`, unik `(dokumen, version_no)`!); arsip-soft (`archived_at`, ganda→404!); process-antre (`queued:true` + `setImmediate`!). Detail/list tak-saring-arsip-kecuali-filter-status! |
| BR-15 | Status-dokumen-tulis-selalu-`ready` (menipu! KI-138; default-DB `processing` tak-pernah-terlihat!); versi `pending→processing→done` (+`skipped`/`failed` khusus-embedding!); PDF-tanpa-teks = `rawText` null → pipeline-skip-diam status-tetap-`pending`!; update-tanpa-file = buffer-dari-judul/ringkasan (bisa-kosong → skip-diam!); audit create/update/archive (`id_branch:null`, archive-`before:{status:'ready'}`-hardcode!); chunk-lama-dihapus-tiap-run (idempoten!); `token=ceil(len/4)`; `metadata` selalu-null! |
| BR-16 | Chunk-800/150-buang-`≤20`-belah-paksa-`>1200` + embedding-per-chunk (`text-embedding-3-small`, potong-8000, gagal-per-chunk-lanjut!) else `skipped`; retrieval embedding-cosine-top5 (dokumen-tak-arsip + `embedding_status IN (done,skipped)` + `embedding NOT NULL`!) else keyword (kata-`>2`-maks-5, `OR LIKE`, ambil-`topK*2`, skor-cocok/total, top-K); indeks FULLTEXT migrasi-004 tak-dipakai-kode!; asisten `topK=4` + 5-dokumen-siap (`Like('ready')`, `take:5`!); `hasEmbeddings = some(score<1)`; policy tanpa-dokumen/chunk/AI-gagal punya pesan-masing-masing! |

## 5. Katalog Pesan Lengkap (teks apa adanya)

**Asisten — format jawaban:**
`Penjualan {label} untuk {cabang}:` + `- Total penjualan: {IDR}`/`- Jumlah order: {n}`/`- Rata-rata per order: {IDR}` ·
`Order pending untuk {cabang}:` (`{i}. {nomor} - {pihak} - {IDR}`) / `Tidak ada order pending untuk {cabang}.` ·
`{Order draft|Order dikonfirmasi|Order diproses|Order selesai|Order dibatalkan|Order aktif|Order terbaru} {label} untuk {cabang}:` (`{i}. {nomor} - {pihak} - {status} - {IDR}`) / `{Tidak ada order draft|…} {label} untuk {cabang}.`/`Belum ada order {label} untuk {cabang}.` ·
`Stok kritis untuk {cabang}:` (`{i}. {nama} ({kode}) tersedia {n}, minimum {n}`) / `Tidak ada stok kritis untuk {cabang}.` ·
`Snapshot performa hari ini untuk {cabang}:` (`- Total penjualan: {IDR}`/`- Jumlah order: {n}`/`- Order pending: {n}`/`- Stok kritis: {n} item`) ·
`Info produk untuk {cabang}:` (`- Nama:…`/`- Kode:…`/`- Status:…`/`- Tersedia: {n} {uom}`/`- Minimum stok: {n} {uom}`/`- Satuan beli/jual: {p}/{s}`/`- Ringkasan:…` opsional) / `Saya belum menemukan produk yang cocok dengan kata kunci "{q}".` ·
`Ringkasan operasional {label} untuk {cabang}:` (4-baris) ·
policy-a (`Belum ada dokumen knowledge yang siap. Upload dokumen SOP atau kebijakan dulu dari tab Basis Pengetahuan di web.`) / policy-b (`Dokumen knowledge tersedia tapi sedang dalam proses indeksasi. Coba tanyakan beberapa saat lagi.`) / policy-c (`Berdasarkan dokumen internal:` + 3 kutipan potong-400+`…`) / AI (`generateRagAnswer`, system: `Kamu adalah asisten internal perusahaan…`, kosong → `Tidak dapat menghasilkan jawaban AI. Pastikan OPENAI_API_KEY dikonfigurasi.`) ·
help 14-baris (`WA Assistant Mini ERP bisa membantu membaca data operasional dari sistem.` … `Kirim /help kapan saja untuk melihat menu ini lagi.`) ·
suffix (`Jika perlu, saya bisa lanjutkan dengan cabang atau periode yang lebih spesifik.` — hanya AI!).

**WA (8 validasi/otorisasi + rate + gateway + format-putus):**
`Nomor WhatsApp tidak valid` (409 create/update) · `Nomor {n} sudah terdaftar` (409 duplikat) · `Nomor WhatsApp tidak ditemukan` (404 update/revoke — update-tanpa-id-valid juga ini!) · `Nomor WhatsApp pengirim tidak valid` (403 inbound-digit) · `Nomor belum diotorisasi untuk memakai asisten` (403 inbound-otorisasi) · `Kanal WhatsApp belum terhubung` (403 simulate-tanpa-connected) · `Terlalu banyak pesan dalam waktu singkat. Silakan tunggu sebentar.` (rate-nyata, dikirim-via-gateway!) · default-nama `Nomor Terotorisasi` ·
gateway: `Gagal memulai gateway WhatsApp[: pesan]` (boot-gagal) · `Koneksi WhatsApp terputus` (default-putus) · `Gateway WhatsApp gagal memproses event[: pesan]` (event-error → `disconnected`!) · format-putus `"{message} | status {n} | reason {r} | data {...} | auth lokal direset, … / mencoba reconnect otomatis"`.
**Knowledge (4 pesan!):** `Nama file wajib diisi` (400) · `Judul dokumen wajib diisi` (400) · `Dokumen knowledge tidak ditemukan` (404 detail/archive) · `Dokumen tidak ditemukan` (404 update/process!) · `Tidak ada versi dokumen ditemukan` (404 process-tanpa-versi).
**UI:** 60+ string (ui-ux-spec §2–4!) — toast slice 15 (lihat F-06 slice!), validasi panel, notice mode/knowledge.

## Temuan baru & pertanyaan terbuka (belum ber-ID)

- Preview vs inbound beda normalisasi mode: preview hanya `ai_assisted`-persis (`assistant.service.ts:577-579`), config/inbound terima `ai`/`hybrid` (`whatsapp.service.ts:486-489`). Akibat: `mode:"ai"` via `assistant/preview` jatuh ke `rule_based` diam-diam meski config `ai`. Cek niat!
- Preview tak-cek kuota harian, inbound cek kuota (`assistant.service.ts:581-584` vs `whatsapp.service.ts:519-549`). Preview superadmin bisa melewati `ai_daily_limit`. Cek niat!
- `resolveEffectiveMode` memakai `Number(prefs.ai_daily_limit ?? 50)` (`whatsapp.service.ts:530`): bila tersimpan string-non-numerik/NaN maka `usedToday >= NaN` selalu false → kuota lolos-terus. Cek validasi tulis config!
- `getProductInfo` memakai `.orderBy('p.productName', 'ASC')` (`tools.service.ts:270`): properti relasi camel, bukan kolom DB — cek apakah order diabaikan/error-di-dialect!
- `GetCriticalStock` tanpa `take/limit` SQL (`tools.service.ts:191-235`): seluruh baris kritis dimuat lalu `slice(0,5)` di format dan `items.length` untuk komposit — cek beban bila ribuan SKU kritis!
- `GetTodayPerformance`/`GetOperationalSummary` menghitung pending/kritis dari `items.length` (limit-5!) (`tools.service.ts:249-254,303-315`): dashboard-BOT menampilkan maks 5 walau data 50. Sama untuk `getSalesSummary` yang benar-SQL. Cek niat cap-diam!
- Thread lama tak-diupdate cabang: `findOrCreateThread` hanya set `idBranch` saat buat (`whatsapp.service.ts:399-431`); pesan simulate cabang-B lalu cabang-C ke nomor sama tetap pakai thread/cabang-lama untuk histori. Cek niat!
- `waitForQrOrConnection` timeout-diam 8 detik (`whatsapp-gateway.service.ts:257-267`): connect return sukses-tanpa-QR; UI hanya tahu via polling-3-detik. Cek kontrak timeout!
- `handleConnectionUpdate` saat QR menulis `sessionStatus:'disconnected'` (`whatsapp-gateway.service.ts:189-192`): negara `reconnecting` hilang saat QR terbit — UI mengandalkan `showQr`, bukan state. Cek niat!
- `extractRawText` hanya txt/md/csv/json (`knowledge.service.ts:218-224`): pdf/docx/xlsx selalu fallback/ringkasan-UI — tanpa pesan ke pemanggil; UI tetap badge `Siap`. Cek niat vs OCR!
- `runPipeline` skip-tanpa-teks return-dini tanpa ubah status (`knowledge.service.ts:334-339`): versi macet `pending/pending` + dokumen `ready` selamanya. Cek status-jujur rebuild!
- Unik `(id_company, phone_e164)` ada di migrasi 003 (`003_phase3_assistant_mvp.sql:14`) tapi entity tanpa `@Unique` — cek! mengandalkan DB + cek-kode (balapan-tulis bisa 500-DB!).
- `audit.archive` menulis `before:{status:'ready'}` hardcode (`knowledge.service.ts:176`): arsip dari status-apa-pun tercatat `ready`. Cek niat!
- `sendText` return-diam bila tanpa-sock (`whatsapp-gateway.service.ts:92-96`): rate-reply/outbound-nyata bisa hilang-diam bila sesi-hilang-di-tengah. Cek retry!
- JID LID numerik (`parsePhoneFromJid`, `whatsapp.service.ts:462-466` + gateway `:457-461`): `@lid` numerik lolos-validasi-digit dan bisa collide dengan nomor-MSISDN. Cek pemisah `@s.whatsapp.net` vs `@lid`!
