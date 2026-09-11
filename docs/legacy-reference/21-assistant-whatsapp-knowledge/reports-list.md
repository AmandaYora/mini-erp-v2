# Reports List — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A.** Modul ini menghasilkan jawaban + agregat run (bukan file!).
Sumber: baca penuh 3 controller + `assistant.service` + `tools.service` +
`knowledge.service` + `whatsapp.service` + slice. Format jawaban persis =
business-rules §5; skema tabel = data-model-legacy. Ambig = **cek!**.

| # | Keluaran | Sumber | Logika |
|---|---|---|---|
| 1 | Jawaban 9-intent | 8 tools + RAG | Format-tetap per intent + cabang + tanggal (lihat F-01!) |
| 2 | Stats 7-hari | `assistant_runs` | Per (hari,intent,mode): total + sukses + rata-ms (GET!) |
| 3 | Daftar dokumen | `knowledge_documents` | Cari + status + 20/100 |
| 4 | Detail dokumen | Dokumen + versi-terbaru | Metadata + teks-mentah + status-tahap |
| 5 | Daftar otorisasi | `whatsapp_authorizations` | Aktif-default (+revoked!) |
| 6 | Status kanal | Channel + sesi | State + nomor + QR + error |
| 7 | Config asisten | `company_settings` | Mode + preferensi |

Detail kontrak tiap keluaran:

1. **Jawaban 9-intent** (`assistant/preview`, `messages/simulate`, inbound-nyata).
   Envelope `{run_id, branch_id, branch_name, intent_type, resolution_mode, answer_text}`.
   Isi `answer_text`: sales 3-baris IDR (`id-ID`, 0-desimal!) / pending ≤5 /
   status-heading-kosong / kritis ≤5 (`tersedia/minimum`) / performa-hari-ini 4-baris /
   produk 7-baris / operasional 4-baris / policy-a/b/c + kutipan-3×400 / help 14-baris /
   suffix-AI (hanya `bot_ai`!). Cabang = override/`substring`/seluruh-perusahaan;
   tanggal = kemarin/7-hari/bulan-ini/hari-ini (performa selalu-hari-ini!).
2. **Stats 7-hari** (`GET assistant/runs/stats`, `whatsapp.view`).
   `{stats:[{day (DATE!), intent_type, resolution_mode (bot_ai/bot_rule!),
   total (COUNT!), success (SUM completed!), avg_duration_ms (AVG mikro→ms, bulat,
   null-bila-tanpa-completed!)}]}`. Window `DATE_SUB(CURDATE(), INTERVAL 7 DAY)`,
   grup hari+intent+mode, urut `day DESC, total DESC`. Tanpa-filter/pagination/UI!
   Dipakai observability (E2E 10 baca `data.stats` array!).
3. **Daftar dokumen** (`POST knowledge/documents/list`, `knowledge.view`).
   `{items:[toResponse], meta:{page (??1), limit (min(??20,100)), total}}`.
   Cari `title/file_name/summary LIKE %q%`; status `archived→arsip-bukan-null,
   active→null, lain→status=…`; urut `updatedAt DESC`. Slice minta `{limit:100}`!
   UI: judul + badge `Siap/Diproses/Gagal/Diarsipkan` + `Tipe · tanggal`.
4. **Detail dokumen** (`POST knowledge/documents/detail`, `knowledge.view`).
   `toResponse + {raw_text (versi-terbaru, null-bila-pdf-tanpa-teks!),
   chunking_status, embedding_status, source_uri, tags[]}`. 404
   `Dokumen knowledge tidak ditemukan`. Tanpa-UI (halaman hanya KV
   Judul/Tipe/Diunggah-oleh/Tanggal/Status/Ringkasan!).
5. **Daftar otorisasi** (`POST whatsapp/authorizations/list`, `whatsapp.view`).
   `{items:[{id, user_name, phone, sender_phone (=phone!), access_level, status,
   is_primary_owner, last_seen_at}]}`. Default aktif; `include_revoked:true` =
   semua; urut `isPrimaryOwner DESC, createdAt DESC`. Slice kirim `{}` (=aktif-saja!)
   → revoked tak-tampil-UI walau badge `Dicabut` ada! UI tabel Nama/Nomor/Akses/
   Status/Aksi + kosong `Belum Ada Nomor`.
6. **Status kanal** (`POST whatsapp/status` + connect/disconnect).
   `{state (connected/reconnecting/disconnected — QR-terbit = disconnected!),
   phone (=bot_phone!), bot_phone, updated_at, provider (baileys!),
   last_error_text (format-putus/boot/event, null-sehat!), qr_data_url (memori,
   null-restart/belum-terbit!)}`. UI 4-keadaan (connected/QR/reconnecting/
   disconnected) + polling-3-detik + `Diperbarui=formatDateTime(updated_at)`.
7. **Config asisten** (`POST whatsapp/assistant-config/get|update`).
   `{mode (rule_based/ai_assisted, normalisasi-alias!), behavior_preferences ({})}`.
   Tulis-merge `whatsapp_mode` + behavior; `ai_daily_limit` (default-50!) hanya
   via API (tanpa-UI!). UI hanya select-mode + 2-notice (efektif/kuota-tak-tampil!).

Yang bukan-laporan (tak-ada-unduh/cetak!): thread/pesan (tak-ada-endpoint-list!),
run/tool (tak-ada-endpoint-list, hanya stats!), versi/chunk (tak-ada-endpoint,
hanya via detail-status!), file-sumber (private, tanpa-URL-unduh-UI!).
