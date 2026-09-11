# Numbering & Sequence — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A. Tidak ada penomoran dokumen di modul ini** (verifikasi: tanpa
sequence/counter/format nomor di 4 modul API maupun web).

Nilai teknis terkait-angka: `version_no` (+1 per ubah!); `sequence_no` tool
(mulai-0 +1!); `chunk_index`; `token_count` (`ceil(len/4)`); `ai_daily_limit`
(50!); rate 10/menit; timeout 15-detik; QR 8-detik; tren? (tak-ada!); top-K
4/5/5-dokumen.

Rincian (semua dari kode!):

| # | Angka | Nilai | Lokasi |
|---|---|---|---|
| 1 | `current_version_no` / `version_no` | Mulai 1, +1 tiap `updateDocument` (unik `(dokumen, version_no)`!) | `knowledge.service.ts:95,258,276` |
| 2 | `sequence_no` tool | Mulai 0, `+=1` sebelum-coba → baris-pertama = 1 (praktis selalu-1!) | `assistant.service.ts:137-141` |
| 3 | `chunk_index` | 0,1,2… per-versi (unik `(versi, index)`!; hapus-per-versi-tiap-pipeline!) | `knowledge.service.ts:349-359` |
| 4 | `token_count` | `Math.ceil(text.length/4)` per-chunk (estimasi! bukan-tokenizer!) | `knowledge.service.ts:355` |
| 5 | Chunking | 800 char, overlap 150, buang `≤20`, belah-paksa `>1200` (potong-800/geser-650) | `knowledge.service.ts:478-506` |
| 6 | Embedding-input | `text.slice(0,8000)` per-chunk | `knowledge.service.ts:536` |
| 7 | Retrieval top-K | Default 5; asisten 4; keyword ambil `topK*2` lalu potong-topK | `knowledge.service.ts:394,326`; `tools.service.ts:326` |
| 8 | Keyword | Kata `len>2`, maks-5 | `knowledge.service.ts:436-440` |
| 9 | Kutipan policy | Maks-3, potong-400 + `…` | `assistant.service.ts:562-565` |
| 10 | `listReadyDocuments` | `take:5` (asisten butuh-5-dokumen-siap!) | `knowledge.service.ts:181-187` |
| 11 | List dokumen | `page??1`, `limit=min(??20,100)`; slice minta 100! | `knowledge.service.ts:44-45` |
| 12 | Limit tool-SQL | `min(limit??5,20)` (pending/byStatus); kritis tanpa-limit! | `tools.service.ts:67,129` |
| 13 | Cap tampil | `slice(0,5)` kritis; pending alami-5; komposit `length`! | `assistant.service.ts:267`; `tools.service.ts:249-254` |
| 14 | `ai_daily_limit` | Default 50 (`Number(prefs.ai_daily_limit ?? 50)`; NaN-lolos!) | `whatsapp.service.ts:530` |
| 15 | Rate WA | 10/menit/jendela-geser per `company:phone` (ke-11-ditolak!) | `whatsapp.service.ts:19-20,496-512` |
| 16 | QR/connect | Tunggu-QR 8000 ms (poll-250 ms); `qrTimeout` 60 s; `connectTimeoutMs` 30 s; `defaultQueryTimeoutMs` 60 s; reconnect-tunda 1500 ms | gateway `:38,151-153,250,257-267` |
| 17 | AI chat | Timeout 15 s (`AbortController`); `max_tokens:600`; `temperature:0.2`; model `OPENAI_CHAT_MODEL ?? gpt-4o-mini` | `assistant.service.ts:610,621,635-636` |
| 18 | Telepon | `^\d{8,15}$` sesudah-buang-nondigit (UI + API sama!) | `whatsapp.service.ts:482-484`; panel `:19-24` |
| 19 | Stats | Window 7-hari (`DATE_SUB(CURDATE(), INTERVAL 7 DAY)`); `avg` bulat (`Math.round`, null-bila-kosong!) | `assistant.service.ts:94,107` |
| 20 | Socket-versi | 3-angka-positif (`parseBaileysVersion`); env `WHATSAPP_BAILEYS_VERSION` | gateway `:350-361` |
| 21 | Sesi/QR UI | Polling-3-detik; img-QR 220 px | panel `:107-113,157` |
| 22 | ID auto | 9× `AUTO_INCREMENT` INT (PK masing-masing!) | migrasi 003 |
| 23 | Tanpa-tren/sequence-dokumen | Tak-ada nomor-order/invoice/antrean-DB di modul ini! | — (verifikasi!) |
