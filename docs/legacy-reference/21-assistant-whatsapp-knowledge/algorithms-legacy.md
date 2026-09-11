# Algorithms Legacy (Konsep) — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok B — ide algoritma**, bukan kode. Konstanta persis dari kode;
langkah dinomori sesuai urutan eksekusi. Klaim meragukan ditandai **cek!**.

## 1. Peta Algoritma → Lokasi

| Algoritma | Lokasi legacy | Invariansi rebuild |
|---|---|---|
| Intent-berurutan + rentang | `assistant.service` | 9-intent-menang-berurutan; kemarin/7-hari/bulan/hari |
| Mode-efektif + fallback | assistant + whatsapp-config | key + kuota-50 → AI else rule; gagal-AI → kutipan (policy!) |
| 8 tools baca | `tools.service` | Satu-per-intent + komposit; cabang-resolusi; cap-5-diam |
| Pairing-Baileys + QR | `whatsapp-gateway` | Reset + QR-8-detik + nomor-dari-socket + reconnect-selektif |
| Gerbang-otorisasi | `whatsapp.service` | Digit-valid + aktif-SEBELUM-run + rate + thread + simpan |
| Pipeline-chunk-embed | `knowledge.service` | 800/150 + per-chunk + skip-tanpa-key + idempoten-hapus |
| Retrieval-2-lapis | `knowledge.service` | Cosine-top5 else keyword-5-kata; asisten-top4 |

## 2. A-01 Intent routing + rentang (`assistant.service.ts:365-438`)

```
normalized = query.toLowerCase()
JIKA match ^(/help|help|bantuan|menu|fitur)$ ATAU /apa saja.*(fitur|bisa)|cara pakai/ → help
ELSE JIKA /sop|kebijakan|policy|aturan/ → policy_qna            # satu-satunya jalur AI
ELSE JIKA /ringkasan|summary|kondisi bisnis|operasional/ → operational_summary
ELSE JIKA /penjualan|sales|omzet|revenue/ → sales_summary
ELSE JIKA /pending|menunggu|belum diproses|belum selesai/ → pending_orders
ELSE JIKA kata-order(order|pesanan|transaksi) DAN kata-status → order_status
ELSE JIKA /stok kritis|stok menipis|restock|stok tipis/ → critical_stock
ELSE JIKA /performa|performance|hari ini/ → today_performance   # menelan "hari ini" umum!
ELSE JIKA /produk|barang|item/ → product_info
ELSE → help
```

Rentang: `kemarin` → H-1 00:00:00.000–23:59:59.999 (`kemarin`); `minggu ini`/`7 hari`
→ H-6 00:00–kini (`7 hari terakhir`); `bulan ini` → tgl-1 00:00–kini (`bulan ini`);
else hari-ini 00:00–23:59 (`hari ini`). Dipakai sales/order-status/operasional;
performa selalu hari-ini (`new Date()` + paksa 00:00–23:59 di tool!).

Status-filter: draft→`(pending,draft)`; konfirmasi→`(active,confirmed)`;
proses→`(active,in_progress)`; selesai→`(completed,completed)`;
batal→`(cancelled,cancelled)`; aktif→`(active,∅)`; else `(∅,∅)` +
`Order terbaru`/empty-`''`. Produk-query: `"..."` pertama else strip
`info|detail|status|stok|produk|barang|item` + rapikan-spasi.

## 3. A-02 Mode efektif + fallback AI (`assistant.service.ts:532-584`, `whatsapp.service.ts:519-549`)

```
preview.requested = (mode === 'ai_assisted') ? ai : rule        # alias ai/hybrid = rule!
preview.executable = (requested==ai DAN OPENAI_API_KEY.trim()) ? ai : rule
inbound.effective = config.mode ai? key? COUNT(bot_ai, today-00:00→kini) < Number(ai_daily_limit ?? 50) ? ai : rule : rule
policy_qna:
  tanpa-dokumen-siap → pesan-upload (mode-tetap!)
  chunk-0 → pesan-indeksasi (mode-tetap!)
  mode-ai → generateRagAnswer(timeout 15s, gpt-4o-mini, 600 token, temp 0.2,
           system "Kamu adalah asisten internal…") → ok: mode ai; gagal/429/kosong: JATUH ke kutipan + mode rule!
  mode-rule → "Berdasarkan dokumen internal:" + ≤3 kutipan (potong 400 + …)
render: ai → answer + "\n\nJika perlu, saya bisa lanjutkan dengan cabang atau periode yang lebih spesifik."
```

Durasi run = `completedAt - startedAt` (DB, dipakai stats); durasi tool =
`Date.now() - started` (JS, `duration_ms`!). Stats SQL: `COUNT(*)` total,
`SUM(status='completed')` sukses, `AVG(TIMESTAMPDIFF(MICROSECOND,started,completed)/1000)`
rata-ms (null bila tanpa-completed → `Math.round` else null!), window
`started_at >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)`, grup (hari,intent,mode).

## 4. A-03 Tools baca + resolusi cabang (`tools.service.ts:25-339`)

```
resolveBranchContext: override? cari (id,idCompany) → ya/tidak-tanpa-error
  else branches(idCompany).find(name/code/city substring query-lower) else {null,null}
sales: sales + !arsip + grup!=cancelled + rentang → SUM/COUNT (+avg)
pending: grup=pending + !arsip, urut due,order ASC, take min(limit??5,20) → items(party '-' bila null)
byStatus: !arsip + filter-opsional(grup,kode,tanggal) + urut order DESC,created DESC, take sama
critical: tracked + !arsip-produk + (default: min NOT NULL AND avail<=min else avail<=threshold)
  + urut avail ASC,nama ASC, TANPA-limit! → asisten slice(0,5); komposit pakai .length!
today: Promise.all([sales-hari-ini, pending-5, critical-semua]) → {totalSales,orderCount,pendingOrderCount:.length,criticalStockCount:.length}
product: exact product_code=trim-upper DULU else LIKE %nama%; !arsip; orderBy p.productName (cek!);
  null→tak-ketemu; ya→ SUM(avail/on_hand) per-cabang-atau-semua
operational: Promise.all([sales-rentang, pending-5, critical-semua])
retrieveKnowledge: retrieve(…,4) + listReadyDocuments → {chunks,hasDocuments,chunkCount,hasEmbeddings:some(score<1)}
getPolicySupportStatus: listReady → {readyCount,titles} — MATI (tak-dipanggil!)
```

Mata-uang: `Intl('id-ID',{style:'currency',currency:'IDR',maximumFractionDigits:0})`.

## 5. A-04 Pairing Baileys + QR (`whatsapp-gateway.service.ts:71-317`, `whatsapp.service.ts:45-64`)

```
connect: ensureChannel → belum-connected? resetForPairing + startGateway + tunggu-8-detik
resetForPairing: tutup-socket(qr_pairing_restart + ws.close, best-effort) + hapus-Map + rm -rf/mkdir company-{id} + disconnected
boot: import-dinamis(@whiskeysockets/baileys) + useMultiFileAuthState(dir) + makeWASocket({
  markOnlineOnConnect:false, syncFullHistory:false, browser:ubuntu/Chrome,
  qrTimeout:60s, connectTimeoutMs:30s, defaultQueryTimeoutMs:60s, getMessage:()=>undefined,
  version: env-WHATSAPP_BAILEYS_VERSION else fetchLatest(jika isLatest) else default-paket, logger:silent })
  → hasConnected = !!creds.me → state reconnecting/disconnected
event qr: qrDataUrl = toDataURL(qr) [memori!] + disconnected
event open: nomor = digit(sock.user.id) → connected + displayNumber?(kondisional!) + lastConnectedAt + reset-authCount + hasConnected=true
event close: info(statusCode,message,reason,data≤8-primitif); reset-auth bila {401,403,405,419,411}∪{500} (hapus-storage!);
  reconnect bila !manual DAN (515 ATAU (hasConnected DAN bukan {401,403,405,419,411,440,500})) → 1500ms-boot-ulang else hapus-sesi;
  tulis "msg | status n | reason r | data {} | auth lokal direset…/mencoba reconnect otomatis"
tunggu-QR: poll-250ms s.d. 8000ms hingga qrDataUrl ATAU connected (timeout-diam!)
disconnect-manual: logout(manual_disconnect, abaikan-gagal) + hapus-Map + rm-rf/mkdir + disconnected + error-null
status: {state, phone=bot_phone=displayNumber??'', updated_at, provider, last_error_text, qr_data_url(memori)}
```

## 6. A-05 Gerbang otorisasi + inbound (`whatsapp.service.ts:268-512`)

```
nyata (Baileys notify): skip fromMe/grup-@g.us/status@broadcast → teks(conversation|extended|caption-img|vid).trim()
  → phone = digit(JID.split('@')[0].split(':')[0]) → skip bila teks/phone kosong → process(sendViaGateway=true, idBranch=null)
process: phone=buang-nondigit; invalid → 403-pengirim-tak-valid; auth-aktif? else 403-belum-diotorisasi;
  !sendViaGateway DAN !connected → 403-belum-terhubung (simulate!; nyata-bebas!);
  sendViaGateway DAN rate? (sliding in-memori 10/60s per company:phone; reset-bila≥60s; ke-11 → sendText-rate + {answer,run:null,branch:null});
  config → effectiveMode(kuota); thread-open-per-(company,kanal,auth,phone) (buat+dengan-idBranch-input; lama-tak-update!);
  lastMessageAt=now; inbound(received,now); lastSeenAt=now;
  try: assistant.handleWhatsappMessage → inbound processed + outbound processed(+runId) + (nyata? sendText(remoteJid,jawaban)) → reply
  catch: inbound failed + rethrow (nyata-ditelan-pemanggil, simulate-diteruskan!)
simulate: process(phone,message_text,provider_id?,id_branch??null,{simulated:true},sendViaGateway=false)
```

Otorisasi-tulis: create — normalisasi→valid?→cek-aktif-duplikat→hitung-utama
(`flag-request ATAU (owner DAN tanpa-utama-aktif)`)→demote-bila-utama→reaktivasi/buat;
update — id-valid?→cari-AKTIF?→telpon-berikut→valid?→cek-milik-orang?→hitung-utama
(`level==owner DAN (flag ATAU lama-utama ATAU tanpa-utama-lain)`).

## 7. A-06 Pipeline chunk + embedding (`knowledge.service.ts:334-390,478-506`)

```
runPipeline(version): rawText? else log-skip + RETURN (status-tetap-pending!)
  hapus-chunk-versi-ini (idempoten!) → chunking=processing
  chunkText(800,150): paragraf = split(/\n{2,}/).trim.nonempty;
    tiap paragraf: append (muat-bila ≤800 else dorong-current(buang≤20!) + bawa-150-overlap + para)
    + splitOverlong (selama >1200: dorong-800-trim, geser-650)
    akhir: dorong-sisa(>20!)
  simpan(chunks, token=ceil(len/4), embedding=null) → chunking=done
  tanpa-OPENAI_API_KEY → embedding=skipped + return
  embedding=processing → tiap-chunk POST /v1/embeddings(text-embedding-3-small, slice-8000), gagal=null-lanjut
  → done(bila ≥1) else failed
```

Versi: create-1 / update-+1; checksum `sha256(fileBuffer)`; tulis-file-private;
`setImmediate` fire-and-forget (error → console saja!).

## 8. A-07 Retrieval 2-lapis (`knowledge.service.ts:394-474`)

```
retrieve(idCompany, query, topK=5):
  JIKA key: qEmb = embedding(query); JIKA ok:
    muat chunk(ber-embedding, dok-tak-arsip, embedding_status∈{done,skipped})   # skipped-aneh-tapi-kode!
    skor = cosine(qEmb, emb) = Σab/(√Σa²·√Σb²) (panjang-min; nol→0)
    urut-turun → topK → {id,contentText,score,idDocumentVersion,metadata}
    JIKA hasil>0 → RETURN  # query-embedding-gagal → jatuh-keyword!
  keyword: terms = split-spasi → buang-nonalfanum → len>2 → maks-5; kosong→[]
    WHERE (LOWER(content) LIKE %t1% OR …) ambil topK*2 (hanya chunking_status=done, dok-tak-arsip!)
    skor = cocok/total (0–1) → urut-turun → topK
asisten: retrieve(…,4); hasDocuments = listReady>0; hasEmbeddings = some(score<1)
```

FULLTEXT migrasi-004 tak-dipakai (LIKE dipakai!). `listReadyDocuments`:
`status Like('ready')` + tak-arsip + `take:5` + `updatedAt DESC`.

## 2. Keputusan Desain (untuk rebuild)

- **Otorisasi-sebagai-batas-data**: nomor-WA = kredensial-baca-ERP — jangan
  longgarkan tanpa keputusan (risiko-tinggi!).
- **Tanpa-AI-tetap-hidup**: rule + keyword-fallback membuat BOT berguna
  tanpa-biaya-OpenAI — pertahankan lapisan-tanpa-AI!
- **Satu-sesi-per-perusahaan**: BOT = akun-milik-owner, bukan banyak-agen —
  jangan ubah ke multi-session tanpa keputusan.
- **Jejak-bukan-audit**: run/tool-menggantikan audit-log (aturan!) — jangan
  gandakan. Pengecualian: 3 aksi knowledge tetap audit (`id_branch:null`,
  archive-`before`-hardcode!).
- **Sunyi-untuk-asing**: penolakan-tanpa-balasan (KI-137!) — putuskan sadar:
  keamanan-vs-UX.
- **Status-menipu**: `ready`-selalu + skip-diam (KI-138!) — rebuild wajib
  status-jujur.
- **Cap-diam-5**: angka komposit = panjang-array-limit — rebuild putuskan:
  tampilkan `+N lainnya` atau hitung-SQL-penuh!
- **QR-in-memori + sesi-plain + rate-in-memori**: hilang-saat-restart/scale-horizontal
  — rebuild butuh store-persisten bila multi-instance!
