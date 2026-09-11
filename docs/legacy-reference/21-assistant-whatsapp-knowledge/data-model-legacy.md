# Data Model Legacy (Konsep) — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok B — konsep saja**, bukan skema persis. Kolom di bawah disalin dari
migrasi 003 (`003_phase3_assistant_mvp.sql`), migrasi 004 (indeks RAG), dan
9 entity. FK persis = migrasi; entity tak-mendefinisikan relasi (`@ManyToOne`
tak-ada — join manual via id!). Config bukan-tabel-sendiri
(`company_settings.assistant_preferences_json`)!

## 1. Daftar Tabel Konseptual

| # | Tabel | Peran konsep |
|---|---|---|
| 1 | `whatsapp_channels` | Satu-baris-per-perusahaan: provider + nomor-BOT + status + koneksi-terakhir + error |
| 2 | `whatsapp_authorizations` | Whitelist: perusahaan + user-nullable + nomor-unik + nama + akses + status + utama + terlihat-terakhir |
| 3 | `conversation_threads` | Thread per (perusahaan, kanal, otorisasi, nomor!) — per-pengirim, bukan-pelanggan! |
| 4 | `conversation_messages` | Pesan masuk/keluar + id-provider + teks + status (`received/processed/failed`!) |
| 5 | `assistant_runs` | Run: perusahaan + cabang + otorisasi + pesan-masuk + intent + mode + model-null + status + jawaban + alasan-gagal + mulai/selesai |
| 6 | `assistant_tool_executions` | Tool per-run: urutan + nama + status + input/output-JSON + ms + error |
| 7 | `knowledge_documents` | Dokumen: perusahaan + tipe + judul + status + sumber + file + ringkasan + tag + versi-berjalan + pembuat + arsip |
| 8 | `knowledge_document_versions` | Versi: nomor + checksum-file + teks + status-chunk/embedding + waktu |
| 9 | `knowledge_chunks` | Potongan: versi + urutan + teks + embedding-nullable + token + metadata-selalu-null |

### 1.1 `whatsapp_channels` (migrasi 003:19-32 + entity 31 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_whatsapp_channel` INT AI PK | `id` | — |
| `id_company` INT, UNIK | `idCompany` | Satu-baris-per-perusahaan (`uq_whatsapp_channels_company`!); `ensureChannel` buat-`disconnected` bila tak-ada! |
| `provider` VARCHAR(30) DEFAULT `baileys` | `provider` | Selalu `baileys` di kode (tak-pernah-ganti!) |
| `display_number` VARCHAR(30) NULL | `displayNumber` | Nomor BOT dari `sock.user.id`; input-connect-diabaikan! |
| `session_status` VARCHAR(30) DEFAULT `disconnected` | `sessionStatus: connected\|reconnecting\|disconnected` | QR-terbit justru tulis `disconnected`! |
| `last_connected_at` DATETIME(3) NULL | `lastConnectedAt` | Diisi saat `open` saja |
| `last_error_text` TEXT NULL | `lastErrorText` | Format-putus-gateway / boot-error / event-error; `null` saat sehat/connect/disconnect-manual! |
| `created_at` / `updated_at` DATETIME(3) | `createdAt`/`updatedAt` | `updated_at` dipetakan ke UI `Diperbarui` + respons `updated_at`! |

### 1.2 `whatsapp_authorizations` (migrasi 003:1-17 + entity 38 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_whatsapp_authorization` INT AI PK | `id` | Input-update `id_whatsapp_authorization`/`id`! |
| `id_company` INT (FK companies) | `idCompany` | Bagian unik `(id_company, phone_e164)` — `uq_whatsapp_authorizations_phone`! |
| `id_user` INT NULL (FK users) | `idUser` | Nullable; dari `id_user` input (jarang-diisi-UI — UI kirim nama, bukan id-user!) |
| `phone_e164` VARCHAR(30) | `phoneE164` | Digit 8–15 (normalisasi-buang-nondigit!); nama-kolom `e164` tapi isi digit-tanpa-`+`! |
| `display_name` VARCHAR(255) DEFAULT `''` | `displayName` | Default-kode `Nomor Terotorisasi`; UI `Nama pemilik nomor` (wajib-HTML!) |
| `access_level` VARCHAR(30) DEFAULT `authorized_party` | `accessLevel: owner\|authorized_party` | — |
| `status` VARCHAR(30) DEFAULT `active` | `status: active\|revoked` (cek! — tipe `AuthorizationStatus` shared) | Revoke = status, bukan-hapus! |
| `is_primary_owner` TINYINT(1) DEFAULT 0 | `isPrimaryOwner` boolean | Tunggal-per-perusahaan (dijaga-kode, bukan-DB!) |
| `last_seen_at` DATETIME(3) NULL | `lastSeenAt` | Diisi tiap inbound; di-null-kan saat reaktivasi! |
| `created_at`/`updated_at` | — | Urut-list `isPrimaryOwner DESC, createdAt DESC`! |

### 1.3 `conversation_threads` (migrasi 003:34-51 + entity 34 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_conversation_thread` AI PK | `id` | — |
| `id_company` (FK) | `idCompany` | — |
| `id_branch` NULL (FK branches) | `idBranch` | Diisi dari input-simulate/preview-cabang saat buat; thread-lama tak-diupdate! |
| `id_channel` (FK channels) | `idChannel` | Satu-kanal-per-perusahaan → praktis konstanta-per-company! |
| `id_whatsapp_authorization` NULL (FK) | `idWhatsappAuthorization` | Nullable-di-DB; kode selalu isi! |
| `external_phone_e164` VARCHAR(30) | `externalPhoneE164` | Nomor pengirim (bukan nomor-BOT!); indeks `(id_company, external_phone_e164)`! |
| `thread_status` DEFAULT `open` | `threadStatus` | Selalu `open` di kode (tak-ada-close!) |
| `last_message_at` | `lastMessageAt` | Update tiap inbound! |
| `created_at`/`updated_at` | — | — |

### 1.4 `conversation_messages` (migrasi 003:53-71 + entity 40 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_conversation_message` AI PK | `id` | `id_inbound_message` run menunjuk sini! |
| `id_company` / `id_branch` NULL | `idCompany`/`idBranch` | Outbound pakai `reply.branch_id ?? input`! |
| `id_thread` (FK threads) | `idThread` | Indeks `(id_thread, created_at)`! |
| `provider_message_id` VARCHAR(255) NULL | `providerMessageId` | `key.id` Baileys (nyata) / input-simulate / null-outbound! |
| `direction` VARCHAR(20) | `direction: inbound\|outbound` | — |
| `message_type` DEFAULT `text` | `messageType` | Selalu `text` di kode! |
| `message_text` TEXT NULL | `messageText` | Teks-trim (nyata) / mentah (simulate)! |
| `raw_payload_json` JSON NULL | `rawPayload` | Nyata `{provider:'baileys',key}`; simulate `{simulated:true}`; outbound `{...input, runId}`! |
| `processing_status` DEFAULT `received` | `processingStatus: received\|processed\|failed` | Inbound `received`→`processed`/`failed`; outbound langsung `processed`! |
| `sent_at` NULL | `sentAt` | `new Date()` keduanya! |

### 1.5 `assistant_runs` (migrasi 003:73-94 + entity 46 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_assistant_run` AI PK | `id` | Return `run_id`! |
| `id_company` (FK) | `idCompany` | Scope stats! |
| `id_branch` NULL (FK) | `idBranch` | Hasil `resolveBranchContext` (bisa-null = seluruh-perusahaan!) |
| `id_authorization` NULL (FK auth) | `idAuthorization` | Null-untuk-preview! |
| `id_inbound_message` NULL (FK messages) | `idInboundMessage` | Null-untuk-preview! |
| `intent_type` VARCHAR(50) | `intentType` | 9-nilai `detectIntent`! |
| `resolution_mode` VARCHAR(30) | `resolutionMode: bot_ai\|bot_rule` | Bukan `ai_assisted`! ditulis-awal + update-akhir! |
| `llm_model_name` VARCHAR(100) NULL | `llmModelName` | Selalu null! |
| `status` DEFAULT `running` | `status: running\|completed\|failed` | `failed` + rethrow = HTTP-500! |
| `final_response_text` TEXT NULL | `finalResponseText` | Jawaban render-mode (suffix-AI!) |
| `failure_reason` TEXT NULL | `failureReason` | `(error).message`! |
| `started_at` DATETIME(3) | `startedAt` | `new Date()` saat create; dasar stats-7-hari + kuota-harian! |
| `completed_at` NULL | `completedAt` | Diisi dua-jalur selesai/gagal! |
| `created_at` | `createdAt` | — |

Indeks: `(id_company, started_at)` — dipakai stats + kuota!

### 1.6 `assistant_tool_executions` (migrasi 003:96-114 + entity 40 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_assistant_tool_execution` AI PK | `id` | — |
| `id_company` / `id_branch` NULL | `idCompany`/`idBranch` | Ikut cabang-aktif! |
| `id_assistant_run` (FK runs) | `idAssistantRun` | Indeks `(run, sequence_no)`! |
| `sequence_no` INT DEFAULT 1 | `sequenceNo` | Mulai-1 (`+=1` sebelum-coba!), +1-per-tool; praktis selalu 1 (satu-tool-per-run!) |
| `tool_name` VARCHAR(100) | `toolName` | `GetSalesSummary|GetPendingOrders|GetOrderByStatus|GetCriticalStock|GetTodayPerformance|GetProductInfo|GetOperationalSummary|RetrieveKnowledge` |
| `status` VARCHAR(30) | `status: success\|failed` | Tanpa `skipped`! |
| `input_json` / `output_json` JSON NULL | `input`/`output` | Input snake_case (`id_company…`); output hasil-tool; gagal → `output:null`! |
| `duration_ms` INT NULL | `durationMs` | `Date.now()-started` (JS, bukan-DB!) |
| `error_text` TEXT NULL | `errorText` | Pesan-error-tool! |
| `executed_at` DATETIME(3) | `executedAt` | `new Date()` sesudah-tool! |

### 1.7 `knowledge_documents` (migrasi 003:116-135 + entity 47 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_knowledge_document` AI PK | `id` | — |
| `id_company` (FK) | `idCompany` | — |
| `document_type` VARCHAR(50) | `documentType: sop\|policy\|glossary\|guide` (cek! shared-type) | Infer-nama! |
| `title` VARCHAR(255) | `title` | Wajib! |
| `status` VARCHAR(30) DEFAULT `processing` | `status` | DB-default `processing`, kode selalu tulis `ready`/`archived`! |
| `source_uri` TEXT NULL | `sourceUri` | Key storage-private (dua-tahap-simpan!) |
| `file_name` VARCHAR(255) | `fileName` | Wajib; update pakai `basename`! |
| `summary_text` TEXT NULL | `summary` | Dari `summary ?? description`; UI-wajib tapi API-boleh-kosong! |
| `tags_json` JSON NULL | `tags` | Null-bila-kosong! |
| `current_version_no` INT DEFAULT 1 | `currentVersionNo` | +1-tiap-update! |
| `id_created_by` INT (FK users) | `idCreatedBy` | `session.idUser`! |
| `created_at`/`updated_at`/`archived_at` NULL | `createdAt`/`updatedAt`/`archivedAt` | Arsip = status + waktu! |

Indeks: `(id_company, status)` + 004 `(id_company, archived_at)`!

### 1.8 `knowledge_document_versions` (migrasi 003:137-151 + entity 31 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_knowledge_document_version` AI PK | `id` | — |
| `id_company` (FK) | `idCompany` | — |
| `id_knowledge_document` (FK) | `idKnowledgeDocument` | Unik `(dokumen, version_no)`! |
| `version_no` INT | `versionNo` | 1,2,3… |
| `source_checksum` VARCHAR(128) NULL | `sourceChecksum` | `sha256(fileBuffer)` hex-64! |
| `raw_text` LONGTEXT NULL | `rawText` | Null = pipeline-skip! |
| `chunking_status` DEFAULT `pending` | `chunkingStatus: pending\|processing\|done` | — |
| `embedding_status` DEFAULT `pending` | `embeddingStatus: pending\|processing\|done\|skipped\|failed` | `skipped` = tanpa-key! |
| `created_at` | `createdAt` | Tanpa `updated_at` (save-ulang tak-tercatat-waktu!) |

Indeks 004: `(id_knowledge_document, version_no)`!

### 1.9 `knowledge_chunks` (migrasi 003:153-167 + entity 31 baris)

| Kolom DB | Entity | Catatan |
|---|---|---|
| `id_knowledge_chunk` AI PK | `id` | — |
| `id_company` (FK) | `idCompany` | — |
| `id_knowledge_document_version` (FK) | `idKnowledgeDocumentVersion` | Unik `(versi, chunk_index)`!; hapus-per-versi-tiap-pipeline! |
| `chunk_index` INT | `chunkIndex` | 0,1,2… |
| `content_text` LONGTEXT NOT NULL | `contentText` | Potongan 800/150! |
| `embedding_json` JSON NULL | `embedding: number[]\|null` | Null = keyword-saja! |
| `token_count` INT DEFAULT 0 | `tokenCount` | `ceil(len/4)` — estimasi, bukan-tokenizer! |
| `metadata_json` JSON NULL | `metadata` | Selalu null! |
| `created_at` | `createdAt` | — |

Indeks 004: `(id_company, id_knowledge_document_version)` + FULLTEXT `content_text`
(tak-dipakai-kode — retrieval pakai LIKE!).

### 1.10 Config (bukan tabel!)

`company_settings.assistant_preferences_json` (`Record<string,unknown>‖null`,
PK `id_company`): kunci `whatsapp_mode` (+ legacy `mode`), `behavior_preferences`
(object), `ai_daily_limit` (number, default 50). Dibaca-tulis via
`ensureSettings` (buat-baris-kosong!). Tanpa entity-khusus, tanpa histori!

## 2. Hubungan Konseptual (bukan FK persis!)

Kanal 1–N otorisasi/thread; otorisasi 1–N thread/pesan/run; thread 1–N pesan;
run 1–N tool; dokumen 1–N versi 1–N chunk. Config di `company_settings.assistantPreferences`
(bukan-tabel-sendiri!). FK-DB persis (migrasi 003): channels→companies;
auth→companies+users; threads→companies+branches+channels+auth;
messages→companies+branches+threads; runs→companies+branches+auth+messages;
tools→companies+branches+runs; documents→companies+users; versions→companies+documents;
chunks→companies+versions. Tanpa-FK-ke-audit (exempt — kecuali 3 `audit-log`
knowledge dengan `id_branch:null`!).

## 3. Anomali & Catatan Rebuild

- Status-dokumen vs status-versi vs tipe-shared tak-sejajar (`ready` vs
  `done/skipped` vs `processed` — KI-138!): dokumen selalu `ready` (tak-jujur!),
  versi `pending/processing/done/skipped/failed`, UI kenal `Siap/Diproses/Gagal/Diarsipkan`.
- `metadata_json` chunk selalu-null; `llm_model_name` selalu-null; checksum-dari-file
  (bukan-teks!) — file-sama-teks-beda (cek! line-ending) = checksum-sama!
- Thread-per-pengirim janggal untuk customer-service (desain-asisten-pribadi!):
  kunci thread (company, kanal, otorisasi, nomor) — dua nomor = dua thread!
- Unik-nomor ada-di-DB (`uq_whatsapp_authorizations_phone`) tapi entity tanpa-`@Unique`
  — [PERLU KONFIRMASI] balapan-tulis mengandalkan 409-DB (cek! pesan-DB-mentah!).
- `thread_status` selalu `open`, `message_type` selalu `text`, `direction` hanya dua —
  kolom-ada-tak-bervariasi, jangan dihapus tanpa cek!
- Presisi waktu: entity `datetime(6)` vs migrasi `DATETIME(3)` — cek! milidetik-vs-mikrodetik.
- `assistant_runs` menyimpan `resolution_mode` ganda-tulis (awal + akhir) — baca-akhir
  untuk stats/kuota!
- Storage file knowledge-private + sesi-BAILEYS-plain di `storage/` — satu-host,
  hilang-saat-redeploy-container bila volume tak-persisten (cek! Docker-satu-container!).
