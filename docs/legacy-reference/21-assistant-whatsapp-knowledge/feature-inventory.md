# Feature Inventory — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diturunkan dari
kode yang dibaca penuh langsung: `assistant.service.ts` (656 baris),
`assistant.controller.ts`, `tools.service.ts` (340 baris),
`whatsapp.service.ts` (563 baris), `whatsapp-channel.service.ts` (65 baris),
`whatsapp-gateway.service.ts` (462 baris), `whatsapp.controller.ts`,
`knowledge.service.ts` (552 baris), `knowledge.controller.ts`, 9 entity
(assistant 2 + whatsapp 4 + knowledge 3), seluruh web `assistant/` (2 halaman +
4 komponen + hook + `assistant.slice.ts` 189 baris + adapter + registry),
E2E 10 + E2E 19 + E2E 01 (bagian assistant), migrasi 003/004/028. Bagian ambigu
ditandai **cek!**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md) ·
[data-model-legacy.md](data-model-legacy.md) · [algorithms-legacy.md](algorithms-legacy.md)

Sub-area: 21a Assistant & tools · 21b WhatsApp kanal & otorisasi · 21c Knowledge/RAG.

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 18 — assistant 2 (1 POST + 1 GET!), whatsapp 10 (semua POST!), knowledge 6 (semua POST!) |
| GET satu-satunya | `GET assistant/runs/stats` (satu-satunya GET di seluruh API!) |
| Method lain | 17 × POST dengan `@HttpCode(200)`; body selalu `{ data: {...} }` |
| Permission | `whatsapp.view/manage`, `whatsapp.simulate` (superadmin-only, migrasi 028!), `knowledge.view/create/update/archive` |
| Halaman | 2 — `/assistant/setup`, `/assistant/config` (+ `/assistant` redirect + 7 alias-mati!) |
| Tabel | 9 — channel, authorization, thread, message, run, tool-execution, document, version, chunk (+ `company_settings.assistant_preferences_json` untuk config!) |
| Provider | Baileys (`@whiskeysockets/baileys`, import dinamis via `Function('specifier','return import(specifier)')`! — bukan whatsapp-web.js!) |
| AI | OpenAI chat (`gpt-4o-mini` default, 15 detik timeout, 600 token, temp 0.2) + embeddings (`text-embedding-3-small`, potong-8000!) — key opsional; tanpa-key = rule-based + keyword-fallback + `skipped`! |
| Audit | Exempt (jejak = run/tool-execution; knowledge create/update/archive tetap tulis `audit-log` dengan `id_branch: null`!) |
| Penomoran | Tidak ada |
| Aksi yang TIDAK ada | Ubah data via asisten (baca saja!); UI simulate/process/detail/update-dokumen; hapus permanen otorisasi/dokumen; pagination stats; rate-limit UI; tampil mode-efektif/kuota/sisa-kuota; tampil durasi run di UI |

**Karakter modul.** BOT pribadi-owner via WA: satu akun perusahaan jadi BOT
(satu sesi Baileys per perusahaan, direktori `storage/whatsapp-sessions/company-{id}`),
nomor whitelist boleh bertanya data operasional + SOP. Tanpa OpenAI pun hidup
(rule-based + keyword-RAG).

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — 21a Asisten (`assistant/preview` + `runs/stats`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Preview | `whatsapp.simulate` (superadmin saja, migrasi 028!); input teks + cabang-override + mode; output `{ run_id, branch_id, branch_name, intent_type, resolution_mode(bot_rule/bot_ai), answer_text }` |
| F-01.2 | 9 intent (urutan-menang!) | help (`/help`, `bantuan`, `cara pakai`, fallback!) · policy_qna (sop/kebijakan — satu-satunya jalur AI!) · operational_summary (mengalahkan sales!) · sales_summary · pending_orders · order_status (6 sub-status + heading/kosong!) · critical_stock (maks 5!) · today_performance (selalu-hari-ini!) · product_info (`"..."` else strip-kata!) |
| F-01.3 | Rentang tanggal | kemarin / 7-hari (`minggu ini`/`7 hari` → `7 hari terakhir`, from = H-6 00:00:00.000!) / bulan-ini (tgl-1!) / hari-ini (default 00:00:00.000–23:59:59.999!) |
| F-01.4 | Statistik run | GET 7-hari per (hari, intent, mode): total + sukses + rata-ms (bulat, null bila tanpa-completed!) |
| F-01.5 | Run gagal | Tool-error → `failed` + `failureReason` + rethrow (HTTP-500, tanpa jawaban ramah!) |
| F-01.6 | Pencatatan run | Tiap execute tulis 1 baris `running` dulu (intent + `bot_ai`/`bot_rule` awal), lalu update `completed` (+jawaban render-mode) / `failed` (+alasan); `llm_model_name` selalu null!; `id_authorization`/`id_inbound_message` null untuk preview! |
| F-01.7 | Suffix AI | Hanya jawaban `ai_assisted` ditambah `\n\nJika perlu, saya bisa lanjutkan dengan cabang atau periode yang lebih spesifik.` |

#### Kontrak endpoint asisten (2)

| Method + path | Permission | Input (`{ data }`) | Output | Error |
|---|---|---|---|---|
| `POST assistant/preview` | `whatsapp.simulate` | `{ message_text: string; id_branch?: number; mode?: string }` — `id_branch ?? session.idActiveBranch ?? null`; `message_text ?? ''`; mode dinormalisasi (`ai_assisted` else `rule_based`!) | `{ run_id, branch_id, branch_name, intent_type, resolution_mode, answer_text }` | 403 tanpa-izin; 401 tanpa-token; 500 bila tool melempar (run `failed`!) |
| `GET assistant/runs/stats` | `whatsapp.view` | Tanpa parameter (scope = `session.idCompany`!) | `{ stats: [{ day, intent_type, resolution_mode, total, success, avg_duration_ms }] }` — 7 hari kalender via `DATE_SUB(CURDATE(), INTERVAL 7 DAY)`, `GROUP BY` hari+intent+mode, `ORDER BY day DESC, total DESC` | 403/401 saja; tanpa filter/pagination! |

### F-02 — 21a Tools (10 method, satu-per-intent + komposit!)

Semua read-only. Dipanggil sekali per intent (help = nol tool!). Input/output JSON
tool dicatat di `assistant_tool_executions` (`sequence_no` mulai 1, `duration_ms`,
`executed_at`).

| # | Method (`tools.service.ts`) | Dibaca/dipakai | Detail kontrak |
|---|---|---|---|
| F-02.1 | `resolveBranchContext(idCompany, query, override?)` | Setiap execute, sebelum intent! | Override-id → cari `(id, idCompany)`, ketemu = `{id,name}` else `{null,null}` tanpa-error!; else substring `name/code/city` di query-lowercase (pertama cocok menang!) else seluruh-perusahaan `{null,null}` |
| F-02.2 | `GetSalesSummary({idCompany,idBranch?,dateFrom,dateTo})` | `sales_summary` | Order `sales` + `archived_at IS NULL` + `status_group <> 'cancelled'` + rentang `order_date`; join branch+currentStatus; `SUM(total_amount)` + `COUNT`; `averageOrderValue = total/count else 0` |
| F-02.3 | `GetPendingOrders({idCompany,idBranch?,limit?,orderKind?})` | `pending_orders`, komposit | `status_group='pending'`, `archived_at IS NULL`, urut `due_date ASC, order_date ASC`, `limit = min(input??5, 20)`; kolom mentah id/order_number/kind/total/due/statusLabel/branch/party (`'-'` bila null!); filter `orderKind` hanya bila `sales`/`purchase`! |
| F-02.4 | `GetOrderByStatus({idCompany,idBranch?,statusGroup?,statusCode?,limit?,dateFrom?,dateTo?})` | `order_status` | `archived_at IS NULL`, urut `order_date DESC, created_at DESC`, limit sama; filter grup+kode+tanggal opsional; kolom + orderDate/statusCode/statusGroup |
| F-02.5 | `GetCriticalStock({idCompany,idBranch?,thresholdOverride?})` | `critical_stock`, komposit | Produk `archived_at IS NULL` + `stock_tracked=1`; default `min_stock_qty IS NOT NULL AND available <= min` else `available <= threshold`; urut `available ASC, product_name ASC`; **tanpa limit SQL** — asisten `slice(0,5)` saat format! |
| F-02.6 | `GetTodayPerformance({idCompany,idBranch?,date})` | `today_performance` | Komposit `Promise.all([sales-hari-ini, pending-5, critical-semua])`; tanggal dipaksa 00:00–23:59 hari input; output `{totalSales, orderCount, pendingOrderCount: items.length, criticalStockCount: items.length}` — angka pending/kritis = panjang-array, bukan count-SQL! |
| F-02.7 | `GetOperationalSummary({idCompany,idBranch?,dateFrom,dateTo})` | `operational_summary` | Komposit sama dengan rentang intent; output `{sales, pendingOrderCount, criticalStockCount}` |
| F-02.8 | `GetProductInfo({idCompany,idBranch?,productCode?,productName?})` | `product_info` | `productCode` (exact `=`, di-`trim`, di-`toUpperCase` oleh pemanggil!) diprioritaskan; else `productName LIKE %...%`; `archived_at IS NULL`; `orderBy p.productName` (cek! — properti camel, kemungkinan tak-efektif); null bila tak-ketemu → jawaban tak-ketemu!; bila ketemu jumlahkan `SUM(available/on_hand)` lintas-cabang (atau satu cabang bila override!) |
| F-02.9 | `RetrieveKnowledge(idCompany, query)` | `policy_qna` | `knowledge.retrieve(idCompany, query, 4)` + `listReadyDocuments` untuk `hasDocuments`; output `{chunks[{id,contentText,score,idDocumentVersion}], hasDocuments, chunkCount, hasEmbeddings: some(score<1)}` — skor 1 = keyword-fallback! |
| F-02.10 | `getPolicySupportStatus(idCompany)` | **Tak-dipanggil-siapa-pun!** | `listReadyDocuments` → `{readyCount, titles}`; mati-kode, kontrak dipertahankan bila dihidupkan lagi! |

Resolusi cabang: override-id else substring nama/kode/kota else seluruh-perusahaan
(gagal = null tanpa-error!).

### F-03 — 21b Kanal WA (`status`, `connect`, `disconnect`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Satu sesi per perusahaan | `Map<companyId, GatewaySession>` in-memory + baris `whatsapp_channels` (unik `id_company`!); provider selalu `baileys` |
| F-03.2 | Connect | `ensureChannel` + bila belum `connected` → `startPairing` = reset + start + tunggu-QR-8-detik (polling 250 ms; timeout-diam, tetap return status!); input `display_number` **diabaikan** (`_data`!) |
| F-03.3 | QR | `QRCode.toDataURL(qr)` → `qr_data_url` in-memory (hilang saat restart!); UI: `Scan QR Nomor BOT` + `Buka WhatsApp perusahaan > Perangkat Tertaut > Tautkan Perangkat.`; img 220 px |
| F-03.4 | Nomor BOT dari socket | Saat `connection==open`: `sock.user.id` → digit → `displayNumber` + `lastConnectedAt` + `connected`; bila JID kosong nomor lama dipertahankan! (cek! — patch kondisional) |
| F-03.5 | Boot socket | `useMultiFileAuthState(dir)` + `makeWASocket({markOnlineOnConnect:false, syncFullHistory:false, browser: ubuntu/Chrome, qrTimeout:60s, connectTimeoutMs:30s, defaultQueryTimeoutMs:60s, getMessage: undefined, version, silent-logger})`; versi dari `WHATSAPP_BAILEYS_VERSION` else fetch-latest (bila `isLatest`) else default-paket (`WHATSAPP_USE_PACKAGE_DEFAULT_VERSION=true` memaksa default!) |
| F-03.6 | Reconnect selektif | `close` → reset-auth bila kode ∈ {401,403,405,419,411} atau 500 (hapus `company-{id}/` + recreate!); reconnect (1500 ms) hanya bila bukan-manual DAN (kode 515 ATAU (pernah-connected DAN bukan-daftar-hitam {401,403,405,419,411,440,500})); else `disconnected` + hapus-sesi!; pesan error `"{message} | status {n} | reason {r} | data {...} | auth lokal direset … / mencoba reconnect otomatis"` |
| F-03.7 | Disconnect | `sock.logout('manual_disconnect')` (diabaikan-gagal!) + hapus-sesi + `rm -rf` + `mkdir` storage + `disconnected` + `lastErrorText=null` (wajib-scan-ulang!) |
| F-03.8 | Status | `{ state, phone, bot_phone (sama!), updated_at, provider, last_error_text, qr_data_url }` |

#### Kontrak endpoint kanal (3)

| Method + path | Permission | Input | Output |
|---|---|---|---|
| `POST whatsapp/status` | `whatsapp.view` | `{}` | Status di atas (`qr_data_url` dari memori!) |
| `POST whatsapp/channel/connect` | `whatsapp.manage` | `{ display_number?: string }` (diabaikan!) | Status sesudah pairing-8-detik |
| `POST whatsapp/channel/disconnect` | `whatsapp.manage` | `{}` | Status `disconnected` |

### F-04 — 21b Otorisasi pengirim (`authorizations/*`, 4 endpoint!)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Validasi nomor | Normalisasi buang-nondigit (`+62 812…` → `62812…`!); valid `^\d{8,15}$` else `409 Nomor WhatsApp tidak valid` |
| F-04.2 | Level + utama-tunggal | `owner`/`authorized_party` (dua ejaan input diterima: `access_level`/`accessLevel`, `is_primary_owner`/`isPrimaryOwner`, `phone_number`/`phone`, `label`/`user_name`!); utama otomatis bila pertama-`owner` tanpa-utama-aktif; `demote-semua` bila jadi-utama! |
| F-04.3 | Duplikat | Unik `(id_company, phone_e164)` di DB (migrasi 003!) + cek-kode: aktif → `409 Nomor {n} sudah terdaftar`; revoked → reaktivasi-menimpa (nama/akses/utama/`lastSeenAt=null`!) |
| F-04.4 | Update | Hanya baris `active` (`revoked` → `404 Nomor WhatsApp tidak ditemukan`, buat-baru!); id dari `id_whatsapp_authorization`/`id` (non-numerik/≤0 → 404!); ganti-ke-nomor-milik-orang → 409!; utama dipertahankan/dihitung ulang (cek §BR!) |
| F-04.5 | Revoke | Status → `revoked` + lepas-utama (bukan-hapus!); id-tak-ada → 404; tanpa response-body! |
| F-04.6 | Daftar | Default hanya `active` (+`include_revoked=true` untuk semua!); urut `isPrimaryOwner DESC, createdAt DESC`; DTO `{id, user_name, phone, sender_phone (duplikat!), access_level, status, is_primary_owner, last_seen_at}` |
| F-04.7 | UI | tambah/edit/hapus + badge + validasi (`Nomor WhatsApp wajib diisi.` / `Nomor WhatsApp harus 8-15 digit.`) + toast 5 (lihat ui-ux-spec!) |

#### Kontrak endpoint otorisasi (4)

| Method + path | Permission | Input | Output/error |
|---|---|---|---|
| `POST whatsapp/authorizations/list` | `whatsapp.view` | `{ include_revoked?: boolean }` | `{ items: [...] }` |
| `POST whatsapp/authorizations/create` | `whatsapp.manage` | `{ phone_number/phone, label/user_name, access_level/accessLevel, is_primary_owner/isPrimaryOwner, id_user? }` | DTO otorisasi; 409 invalid/duplikat |
| `POST whatsapp/authorizations/update` | `whatsapp.manage` | `{ id_whatsapp_authorization/id, ...field-create }` | DTO; 404 tak-ketemu/revoked; 409 invalid/duplikat |
| `POST whatsapp/authorizations/revoke` | `whatsapp.manage` | `{ id_whatsapp_authorization: number }` | Tanpa body; 404 tak-ketemu |

### F-05 — 21b Inbound & simulate (`messages/simulate` + gateway!)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Filter nyata | Abaikan `type!=='notify`, `fromMe`, grup `@g.us`, `status@broadcast`; ekstrak `conversation`/`extendedText.text`/`image.caption`/`video.caption` (`.trim()`!); stiker/audio/lokasi/dokumen → teks-kosong → abaikan-diam!; JID → digit (`split('@')[0].split(':')[0]`, buang-nondigit; LID-numerik lolos sebagai digit!); kosong → skip! |
| F-05.2 | Gerbang otorisasi | Digit-valid else `403 Nomor WhatsApp pengirim tidak valid`; aktif-wajib (`403 Nomor belum diotorisasi untuk memakai asisten`) **SEBELUM run dibuat**!; revoked/tak-dikenal di jalur nyata = error-ditelan-gateway = senyap-total (KI-137!) |
| F-05.3 | Syarat kanal | Simulate (`sendViaGateway=false`): wajib `connected` else `403 Kanal WhatsApp belum terhubung`; nyata (`true`): bebas-tanpa-cek! (KI-136) |
| F-05.4 | Rate-limit | **Hanya nyata**: sliding-window in-memory per `company:phone`, 10/menit (ke-11 → kirim `Terlalu banyak pesan dalam waktu singkat. Silakan tunggu sebentar.` via gateway + return `{answer_text, run_id:null, branch_id:null}` tanpa-run!); reset-jendela bila ≥60 detik; hilang saat restart! |
| F-05.5 | Mode efektif + kuota | `getAssistantConfig` → `resolveEffectiveMode`: `ai_assisted` + `OPENAI_API_KEY` + `COUNT(bot_ai, hari-ini) < ai_daily_limit (default 50!)` else `rule_based`; preview TIDAK pakai kuota (hanya cek key!) |
| F-05.6 | Thread + pesan | Thread `open` per (company, channel, otorisasi, nomor); buat-bila-tak-ada (`idBranch` input!; thread-lama tak-diupdate-cabang!); `lastMessageAt=now`; inbound `received` → `processed`/`failed`; outbound `processed` + `runId` di `raw_payload_json`; `lastSeenAt=now` |
| F-05.7 | Kirim + error | Nyata: `gateway.sendText(remoteJid, jawaban)`; gagal-per-pesan ditelan (gateway-hidup!); simulate: tanpa-kirim + error-diteruskan ke pemanggil!; tanpa-UI (API saja!) |

#### Kontrak endpoint simulate (1)

| Method + path | Permission | Input | Output |
|---|---|---|---|
| `POST whatsapp/messages/simulate` | `whatsapp.manage` | `{ phone_number, message_text, provider_message_id?, id_branch? }` | Reply asisten `{run_id, branch_id, branch_name, intent_type, resolution_mode, answer_text}`; 403 validasi/otorisasi/kanal |

### F-06 — 21b Config (`assistant-config/get|update`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Baca | `ensureSettings` (buat-baris-kosong bila tak-ada!) → `{ mode: normalize(whatsapp_mode ?? mode ?? 'rule_based'), behavior_preferences: {} }` |
| F-06.2 | Tulis | Merge `assistantPreferences`: `whatsapp_mode = normalize(input.mode ?? tersimpan)`; `behavior_preferences = input ?? tersimpan ?? {}`; return config-baru |
| F-06.3 | Normalisasi mode | `ai`/`hybrid`/`ai_assisted` → `ai_assisted`; asing (termasuk string-kosong/null!) → `rule_based` diam-diam!; preview hanya kenal `ai_assisted`-persis (alias `ai`/`hybrid` via preview = rule!) |
| F-06.4 | UI | select-langsung-simpan + notice-2-mode + warning-tanpa-kelola; mode-efektif & kuota & sisa tak-tampil (KI-141!) |

#### Kontrak endpoint config (2)

| Method + path | Permission | Input | Output |
|---|---|---|---|
| `POST whatsapp/assistant-config/get` | `whatsapp.view` | `{}` | `{ mode, behavior_preferences }` |
| `POST whatsapp/assistant-config/update` | `whatsapp.manage` | `{ mode?: string; behavior_preferences?: object }` | Config baru |

### F-07 — 21c Knowledge (`documents/*`, 6 endpoint!)

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Daftar | Cari `title/file_name/summary LIKE %q%` + status (`archived`→`archived_at NOT NULL`; `active`→NULL; lain→`status=`) + paging `page??1`, `limit=min(??20,100)` + urut `updatedAt DESC`; output `{items[toResponse], meta:{page,limit,total}}` |
| F-07.2 | Buat | `file_name` + `title` wajib (400 `Nama file wajib diisi` / `Judul dokumen wajib diisi`!); tipe-otomatis-dari-nama (`sop`→sop; `policy`/`kebijakan`→policy; `glos`→glossary; else guide!) kecuali dikirim eksplisit; buffer = base64 else fallback `title+summary/description+raw_text`; `rawText` hanya txt/md/csv/json else fallback (pdf/lain tanpa-fallback = null!); `sha256(buffer)`; transaksi dokumen (`ready`! + `currentVersionNo:1` + `idCreatedBy`) + tulis-file-private + versi-1 (`pending/pending`); audit `knowledge.create` (`id_branch:null`!); pipeline `setImmediate` fire-and-forget! |
| F-07.3 | Detail | Dokumen + versi-terbaru (`versionNo DESC`); 404 `Dokumen knowledge tidak ditemukan`; output `toResponse + raw_text + chunking_status + embedding_status + source_uri + tags[]` |
| F-07.4 | Arsip | Hanya non-arsip (arsip-ganda → 404!); set `archived` + `archivedAt`; audit `knowledge.archive` dengan `before:{status:'ready'}` hardcode!; tanpa body! |
| F-07.5 | Update | Hanya non-arsip (404 `Dokumen tidak ditemukan` — pesan beda dari detail!); `versionNo+1`; nama-file sanitize else lama; buffer base64 else fallback (tanpa-file = timpa-dari-judul/ringkasan, bisa-kosong!); sha256; tulis-file; transaksi update-dokumen + versi-baru (`pending/pending`); audit `knowledge.update`; pipeline ulang! |
| F-07.6 | Process | Antre-ulang versi-terbaru: 404 `Dokumen tidak ditemukan` / `Tidak ada versi dokumen ditemukan`; `setImmediate` + langsung `{ queued:true, version_no }`! |
| F-07.7 | UI | daftar-badge + detail + upload-modal (judul/tipe/nama-file/file/ringkasan; ringkasan-wajib di UI!) + arsip + notice-indeks/gagal + toast 4 |

#### Kontrak endpoint knowledge (6, semua POST `@HttpCode(200)`)

| Method + path | Permission | Input | Output/error |
|---|---|---|---|
| `POST knowledge/documents/list` | `knowledge.view` | `{ search?, status?, page?, limit? }` | `{ items, meta }` |
| `POST knowledge/documents/create` | `knowledge.create` | `{ file_base64?, file_name*, title*, description?, summary?, document_type?, tags?, raw_text? }` | `toResponse`; 400 2-pesan-wajib |
| `POST knowledge/documents/detail` | `knowledge.view` | `{ id_knowledge_document }` | Detail + status-tahap; 404 |
| `POST knowledge/documents/archive` | `knowledge.archive` | `{ id_knowledge_document }` | Tanpa body; 404 |
| `POST knowledge/documents/update` | `knowledge.update` | `{ id_knowledge_document, ...field-create }` | `toResponse`; 404 |
| `POST knowledge/documents/process` | `knowledge.update` | `{ id_knowledge_document }` | `{ queued:true, version_no }`; 404 ×2 |

`toResponse` = `{id, title, document_type, status, uploaded_at, id_created_by, summary('' bila null!), file_name, tags([] bila null!), source_uri}`.

### F-08 — 21c Pipeline & retrieval (async!)

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Pemicu | `setImmediate(runPipeline)` sehabis create/update/process; error hanya `console.error`, tak-terlihat-pemanggil! |
| F-08.2 | Skip-tanpa-teks | `rawText` null/kosong → log `has no raw text — skipping` + return (status tetap `pending` selamanya!) |
| F-08.3 | Chunking | Hapus-chunk-lama versi-itu dulu (idempoten!); `chunkingStatus: processing→done`; `chunkText(800/150)`: pecah paragraf `\n{2,}`, gabung s.d. 800, overlap 150 char, buang `≤20`, belah-paksa bila `>1200` (potong 800, geser `800-150`); `tokenCount=ceil(len/4)`; `embedding=null`, `metadata=null`! |
| F-08.4 | Embedding | Tanpa `OPENAI_API_KEY` → `skipped` + log keyword-fallback; else `processing` → per-chunk `POST /v1/embeddings` (`text-embedding-3-small`, `input.slice(0,8000)`), gagal-per-chunk = null (lanjut!); akhir `done` bila ≥1 else `failed`! |
| F-08.5 | Retrieval-embedding | Bila key + embedding-query-ok: muat chunk ber-embedding milik dokumen-tak-arsip + `embedding_status IN ('done','skipped')`; cosine `dot/(|a||b|)` (panjang-min!; nol→0); urut-turun, top-K (default 5) |
| F-08.6 | Retrieval-keyword | Fallback: kata `split(/\s+/)`, buang-nonalfanum, `len>2`, maks-5; kosong → `[]`!; `OR LOWER(content) LIKE %kata%` (ambil `topK*2`); skor = cocok/total (0–1!); urut-turun, top-K; catatan: indeks FULLTEXT migrasi 004 ada tapi query memakai LIKE (FULLTEXT tak-dipakai kode!) |
| F-08.7 | Pemakaian asisten | `retrieve(idCompany, query, 4)`; tanpa-dokumen-siap → pesan-upload; chunk-0 → pesan-indeksasi; `ai_assisted` → `generateRagAnswer` (gagal/timeout-15s/429/kosong → jatuh-ke-kutipan-`rule_based`!); rule → `Berdasarkan dokumen internal:` + maks-3 kutipan (potong-400 + `…`!) |

---

## 3. Edge Case (24, ringkas)

E-01 tanpa-izin → 403 (preview-simulate / manage / knowledge-crud!) · E-02 tanpa-token
→ 401 · E-03 teks-kosong → help · E-04 mode-asing → rule-diam · E-05 cabang-asing →
seluruh-perusahaan · E-06 tanpa-OpenAI → bot_rule-diam · E-07 AI-gagal → fallback-hanya-
policy (intent-lain-tak-terdampak!) · E-08 tool-gagal → run-failed + 500 · E-09 angka-
pending/kritis = panjang-array-limit-5 (KI-139!) · E-10 produk-nama-bukan-kode →
tak-ketemu (KI-140!) · E-11 QR-timeout-8-detik-diam (poll-status!) · E-12 klik-connect-
saat-reconnecting = reset (hapus-storage + boot-ulang!) · E-13 disconnect = scan-ulang ·
E-14 grup/broadcast/milik-sendiri/stiker/audio/lokasi → abaikan-diam · E-15 JID-LID-numerik bisa-collide ·
E-16 revoked/tak-dikenal = senyap-total (KI-137!) · E-17 sim-vs-nyata 7-beda
(filter/cabang/kanal/rate/kirim/error!) · E-18 update-nomor-revoked = buat-baru ·
E-19 ganti-ke-nomor-revoked-orang = ditolak · E-20 kuota-NaN = lolos-terus ·
E-21 PDF-tanpa-teks = skip-diam · E-22 update-tanpa-file = timpa-kosong (KI-138!) ·
E-23 revoke + arsip-dokumen = void-tanpa-data · E-24 tanpa-cabang/store = null.

---

## 4. Katalog Pesan (teks apa adanya; lengkap business-rules §5)

Asisten (format jawaban 9-intent + help + policy-a/b/c + suffix-AI!); WA
(`Nomor WhatsApp tidak valid` · `Nomor {n} sudah terdaftar` · `Nomor WhatsApp
tidak ditemukan` · `Nomor WhatsApp pengirim tidak valid` · `Nomor belum diotorisasi
untuk memakai asisten` · `Kanal WhatsApp belum terhubung` · `Terlalu banyak pesan
dalam waktu singkat. Silakan tunggu sebentar.` · `Nomor Terotorisasi` · gateway
4-pesan + format-putus!); knowledge (4 pesan!); UI (60+ label/toast — ui-ux-spec!).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Ubah data via asisten | Kapabilitas tertulis + tools baca-saja |
| NF-02 | UI simulate/process/detail/update | Slice tanpa panggilan; halaman tanpa form (`assistant.slice.ts`: hanya list/status/config-get + 7 aksi tulis!) |
| NF-03 | Tampil mode-efektif/kuota/sisa | Config hanya mode-tersimpan (KI-141) |
| NF-04 | `whatsapp.simulate` untuk non-superadmin | Default + system-only (migrasi 028!) |
| NF-05 | Hapus permanen otorisasi/dokumen | Revoked/arsip (soft!) |
| NF-06 | Paginasi stats / filter run | GET 7-hari-mentah |
| NF-07 | Validasi DTO preview | Inline-tanpa-validator (terima semua!) |
| NF-08 | Audit untuk modul ini | Exempt (aturan!) — kecuali knowledge create/update/archive tetap audit dengan `id_branch: null`! |
| NF-09 | Enkripsi sesi Baileys | At-rest-plain (KI-142!) — file `creds.json` + key di `storage/whatsapp-sessions/company-{id}/` |
| NF-10 | Rute hidup `/assistant/simulate`, `/whatsapp/*`, `/knowledge/*` | 7 alias-mati + 1 induk, semua `Navigate replace`! |
| NF-11 | Embedding lokal / antrean DB / OCR PDF | Tanpa library PDF; `extractRawText` hanya txt/md/csv/json; pdf = fallback/null! |
| NF-12 | ePOS2 / multi-device / broadcast WA | Satu-sesi-per-perusahaan; grup/broadcast diabaikan! |
| NF-13 | Nilai `llm_model_name` | Selalu null (kolom ada, tak-pernah-diisi!) |
| NF-14 | `metadata_json` chunk | Selalu null saat tulis! |
| NF-15 | `getPolicySupportStatus` dipakai | Mati-kode (tak-dipanggil)! |
