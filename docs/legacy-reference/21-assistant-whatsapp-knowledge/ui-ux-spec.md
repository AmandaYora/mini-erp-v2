# UI/UX Spec — Modul 21 Assistant AI + WhatsApp + Knowledge/RAG

**Kelompok A.** 2 halaman + 4 komponen + hook/slice + rute + adapter. Sumber: baca penuh
web `assistant/` (label disalin persis!): `assistant-setup-page.tsx` (22),
`assistant-config-page.tsx` (34), `assistant-setup-panel.tsx` (388),
`assistant-answer-mode-section.tsx` (53), `assistant-knowledge-section.tsx` (260),
`assistant-capabilities-section.tsx` (95), `use-assistant-module.ts` (32),
`assistant.slice.ts` (189), `org.adapter.ts`, `module-registry.tsx`,
`route-access.test.ts`, E2E 19/10/01. Ambig ditandai **cek!**.

---

## 1. Rute (`module-registry.tsx`)

| Path | Permission | Elemen | Menu |
|---|---|---|---|
| `/assistant` | `whatsapp.view` | `Navigate replace → /assistant/setup` | — |
| `/assistant/setup` | `whatsapp.view` | `AssistantSetupPage` | `Asisten WA > Setup WA Assistant` |
| `/assistant/config` | `whatsapp.view` | `AssistantConfigPage` | `Asisten WA > Config WA Assistant` |
| `/assistant/channel` | `whatsapp.view` | `Navigate replace → /assistant/setup` | — (alias-mati 1!) |
| `/assistant/knowledge` | `knowledge.view` | `Navigate replace → /assistant/config` | — (alias-mati 2!) |
| `/assistant/tools` | `whatsapp.view` | `Navigate replace → /assistant/config` | — (alias-mati 3!) |
| `/knowledge` | `knowledge.view` | `Navigate replace → /assistant/config` | — (alias-mati 4!) |
| `/knowledge/upload` | `knowledge.view` | `Navigate replace → /assistant/config` | — (alias-mati 5!) |
| `/knowledge/retrieval` | `knowledge.view` | `Navigate replace → /assistant/config` | — (alias-mati 6!) |
| `/whatsapp` | `whatsapp.view` | `Navigate replace → /assistant/setup` | — (alias-mati 7!) |

Total 7 alias-mati + 1 induk (`/assistant`) = 8 `Navigate replace`. Judul alias =
`Asisten WA`; halaman hidup `Setup WA Assistant` / `Config WA Assistant`.
`route-access.test.ts` mengunci permission tiap path di atas; `module-registry.test.ts`
mengunci judul alias = `Asisten WA`. Semua rute `shell:true`, `protected`.

## 2. Setup WA Assistant — `/assistant/setup`

Header (`Dashboard > Setup WA Assistant` + `Setup WA Assistant` +
`Hubungkan satu akun WhatsApp perusahaan sebagai BOT dan tentukan nomor pengirim
yang boleh mengaksesnya.`). Tanpa cabang/workspace → null (blank! tanpa spinner!).

**Panel Nomor BOT** (`Akun WhatsApp perusahaan yang menerima dan membalas pesan.`):
4 keadaan — connected (`Terhubung` + nomor + diperbarui + `Putuskan Koneksi`
danger-kelola!) · QR (`Scan QR Nomor BOT` + img 220px else `Menyiapkan QR...` /
`QR Belum Bisa Dibuat` + `QR gagal dibuat` + `Buka WhatsApp perusahaan >
Perangkat Tertaut > Tautkan Perangkat.` / `WhatsApp menolak sesi pairing sebelum
QR diterbitkan. Coba muat ulang QR beberapa saat lagi.` + notice-error + `Tutup`
+ `Muat Ulang QR BOT`) · reconnecting (`Koneksi BOT Belum Selesai` +
`Gateway sedang menunggu pairing akun WhatsApp perusahaan.` + `Tampilkan QR BOT`
+ `Reset Status`) · disconnected (`Nomor BOT Belum Terhubung` + `Tautkan satu
akun WhatsApp perusahaan sebagai BOT.` + `Hubungkan Nomor BOT (QR)`).
Polling-3-detik saat QR-terbuka (auto-tutup-saat-connected!).

Detail perilaku panel (teks persis!):
- Connected: KV `Status` = `Terhubung` (hijau `!text-ok`!); `Nomor BOT` =
  `whatsappChannelStatus.phone || "-"`; `Diperbarui` = `formatDateTime(updatedAt)`;
  tombol `Putuskan Koneksi` (`variant="danger"`) hanya bila `can("whatsapp.manage")`.
- QR (`showQr==true`, prioritas atas `reconnecting`!): kotak putih padding-4 berisi
  `<img alt="QR WhatsApp" class="h-[220px] w-[220px]" src={qrDataUrl}>` bila ada,
  else kotak 220px berisi `Menyiapkan QR...` / `QR gagal dibuat` (`qrFailed =
  showQr && !qrDataUrl && lastErrorText`); judul `Scan QR Nomor BOT` /
  `QR Belum Bisa Dibuat`; instruksi `Buka WhatsApp perusahaan > Perangkat Tertaut
  > Tautkan Perangkat.` / `WhatsApp menolak sesi pairing sebelum QR diterbitkan.
  Coba muat ulang QR beberapa saat lagi.`; `Notice tone="warning"` berisi
  `lastErrorText` bila ada; tombol `Tutup` (secondary, `setShowQr(false)`) +
  `Muat Ulang QR BOT` (hanya-kelola → `connectWhatsappChannel()` tanpa-argumen!).
- `requestQr()`: `setShowQr(true)` + bila tanpa-`qrDataUrl` → `connectWhatsappChannel()`.
  Polling: `useEffect` interval 3000 ms `refreshAssistantData()` selama
  `showQr && state!=="connected"`; efek kedua auto-`setShowQr(false)` saat connected.
- Reconnecting (hanya bila `!showQr`!): mark `WA`, judul `Koneksi BOT Belum Selesai`,
  deskripsi `Gateway sedang menunggu pairing akun WhatsApp perusahaan.`,
  notice-error bila ada, tombol-kelola `Tampilkan QR BOT` (= `requestQr`) +
  `Reset Status` (= `disconnectWhatsappChannel()` — cek! label reset tapi aksi putus!).
- Disconnected: mark `WA`, judul `Nomor BOT Belum Terhubung`, deskripsi
  `Tautkan satu akun WhatsApp perusahaan sebagai BOT.`, tombol-kelola
  `Hubungkan Nomor BOT (QR)`. Tanpa-kelola: panel-kosong tanpa-tombol (cek! viewer
  reconnecting/disconnected tanpa-aksi!).

**Panel whitelist** (`Nomor yang Boleh Mengakses BOT` + `Nomor di daftar ini adalah
pengirim yang boleh mengirim pesan ke nomor BOT.` + info `Pengguna mengirim pesan
dari nomor yang terdaftar di sini ke nomor BOT yang terhubung di atas.` +
`Tambah Nomor`): tabel Nama/Nomor/Akses/Status/Aksi (badge `Terotorisasi/Owner` +
`Aktif/Dicabut`; `Edit` + `Hapus` bila aktif-kelola!) · kosong `Belum Ada Nomor`
(`Belum ada nomor pengirim yang diberi akses ke BOT.`). Modal tambah/edit
(`Tambah/Edit Nomor Pengakses BOT` + `Nomor ini adalah pengirim yang boleh
mengirim pesan ke akun WhatsApp BOT.`; `Nama pemilik nomor*` + `Nomor pengirim
WA` (`6281234567890`, numeric!) + `Level akses` (`Pihak Terotorisasi/Owner`);
`Batal`/`Simpan Nomor`/`Simpan Perubahan`; validasi `Nomor WhatsApp wajib diisi.`
/ `Nomor WhatsApp harus 8-15 digit.`). Modal hapus (`Hapus Nomor Pengakses BOT` +
`Akses nomor ini akan dicabut dari daftar pengirim yang boleh memakai BOT.` +
`{nama} ({nomor}) tidak akan bisa mengirim pesan ke BOT.`; `Batal`/`Hapus Nomor`).

Detail whitelist:
- Tabel kolom persis `Nama/Nomor/Akses/Status/Aksi`; badge akses
  `Terotorisasi` (authorized_party) / `Owner`, tone neutral; badge status
  `Aktif` (success) / `Dicabut` (warning); aksi `Edit` (ghost) + `Hapus` (danger)
  hanya `status==active && canManage`, else sel-kosong!
- Daftar = `whatsappAuthorizations` dari store (sudah-di-map adapter; `sender_phone`
  API diabaikan, `phone` dipakai!); termasuk revoked (API default-aktif tapi slice
  tanpa `include_revoked` — cek! kok revoked tampil? karena list tanpa-filter?
  **cek!** — slice kirim `{}` = aktif-saja, jadi revoked hilang-dari-UI walau badge ada!).
- Modal tambah/edit: `Nama pemilik nomor` (`required`, tanpa-asterisk-di-label!),
  `Nomor pengirim WA` (`inputMode="numeric"`, `placeholder="6281234567890"`,
  `required`, error-inline!), `Level akses` (`SearchableSelect`: `Pihak Terotorisasi`/
  `Owner`); submit → `normalizeWhatsappPhone` (buang-nondigit!) + validasi-front
  (`getWhatsappPhoneError`: kosong → `Nomor WhatsApp wajib diisi.`; len∉8–15 →
  `Nomor WhatsApp harus 8-15 digit.`) → `saveWhatsappAuthorization({id?, accessLevel,
  isPrimaryOwner:false (!), phone, status:"active", userName})` → `then(close)` —
  **cek!** `isPrimaryOwner:false` selalu (UI tak-bisa-set-utama!).
- Modal hapus: `size="sm"`, deskripsi + `{userName} ({phone}) tidak akan bisa
  mengirim pesan ke BOT.`; aksi `Hapus Nomor` (danger) → `revokeWhatsappAuthorization(id)`.

## 3. Config WA Assistant — `/assistant/config`

Header (`Dashboard > Config WA Assistant` + deskripsi `Atur cara BOT menjawab,
dokumen knowledge, dan daftar kemampuan yang bisa digunakan dari WhatsApp.`).
Urutan: mode + knowledge (gated!) + kapabilitas. Guard cabang/workspace sama.

**Cara Menjawab** (`Pilih cara BOT menyusun jawaban.`): kelola = select
(`BOT Standar`/`BOT Dibantu AI`, simpan-langsung!); lihat = `Mode aktif` +
label + warning `Anda hanya punya akses lihat. Perubahan mode BOT memerlukan
izin kelola.`; selalu info (`BOT menjawab memakai pola pertanyaan yang sudah
disiapkan.` / `BOT memakai pola yang sama, AI membantu memahami variasi bahasa
dan dokumen.`).

Detail: `mode` dari `whatsappChannelStatus.mode` (hasil `assistant-config/get`,
default `rule_based`!); label `modeLabel` (`rule_based`→`BOT Standar`,
else→`BOT Dibantu AI`); kelola (`can("whatsapp.manage")`) tampil
`FormField label="Mode jawaban"` + `SearchableSelect id="wa-mode-select"`
(options 2) — `onChange` langsung `saveAssistantConfig({mode})` tanpa-tombol-simpan!;
viewer tampil KV `Mode aktif` + strong-label; notice-info deskripsi-mode selalu;
notice-warning izin hanya-viewer. Kuota/sisa/mode-efektif tak-ada-di-UI!

**Knowledge** (tanpa `knowledge.view` = kartu + warning `Akun Anda belum memiliki
izin melihat atau mengelola knowledge.`): daftar (`Referensi dan panduan yang
dipakai asisten untuk menjawab.` + `Upload`; baris judul + badge
`Siap/Diproses/Gagal/Diarsipkan` + `{Tipe:SOP/Kebijakan/Glosarium/Panduan} ·
{tanggal}`; kosong `Dokumen kosong`/`Belum ada dokumen yang diunggah.`) +
detail (`Metadata dan status dokumen terpilih.`; KV Judul/Tipe/Diunggah-oleh/
Tanggal/Status/Ringkasan; notice `Dokumen sedang diindeks. Biasanya selesai
dalam beberapa menit.` / `Pemrosesan gagal. Coba upload ulang dokumen ini.`;
`Arsipkan Dokumen` danger-bila-bisa; kosong `Belum ada dokumen terpilih`/`Pilih
dokumen dari daftar untuk melihat detail.`) + modal upload (`Upload Dokumen
Baru`; `Judul dokumen*` + `Tipe dokumen` + `Nama file*` (`contoh:
sop-retur-v3.pdf`) + `File dokumen` (auto-nama!) + `Ringkasan isi*`;
`Batal`/`Upload`).

Detail: gate `can("knowledge.view")` else `SectionCard title="Knowledge"
description="Dokumen referensi memerlukan izin knowledge."` + warning di atas.
Daftar `SectionCard title="Daftar Dokumen" description="Referensi dan panduan…"`,
aksi `Upload` (secondary) hanya `can("knowledge.create")`; baris = `<button
class=list-item …>` berisi `<strong>{title}</strong>` + `Badge` status
(`Siap` success / `Diproses` warning / `Gagal` danger / `Diarsipkan` neutral;
map-tanpa-default → status-asing-tampil-mentah!) + sub
`{SOP|Kebijakan|Glosarium|Panduan} · {formatDateTime(uploadedAt)}`; klik
`setSelectedId`. `selectedId` init = dokumen-pertama (`useState` sekali — cek!
tak-ikut-refresh-list!). Detail KV persis `Judul/Tipe/Diunggah oleh
(findUserName(uploadedByUserId))/Tanggal upload/Status/Ringkasan`; ringkasan
pakai `<span>` bukan-strong!; notice-info bila `processing`, notice-danger bila
`failed`; tombol `Arsipkan Dokumen` (danger) bila `can("knowledge.archive") &&
status!=="archived"`. Modal `Upload Dokumen Baru` (tanpa-description!): grid
`Judul dokumen` (required) + `Tipe dokumen` (select 4) + `Nama file` col-span-full
(required, placeholder `contoh: sop-retur-v3.pdf`) + `File dokumen` (type=file,
pilih → `setUploadFileName(file.name)` + FileReader-DataURL → base64-sebelah-koma!)
+ `Ringkasan isi` (textarea 80px, required!); submit → `saveKnowledgeDocument({
title, documentType, status:"processing" (!), uploadedByUserId: userRecords[0] (cek!
user-pertama-bukan-aktif!), summary, fileName, fileBase64?, description:summary })`
→ reset-5-field + tutup. **cek!** `status:"processing"` input diabaikan-API
(API tulis `ready`!).

**Kapabilitas** (statis!): `Yang Bisa Ditanyakan` (4 kartu: ringkasan/pending/
kritis/SOP + contoh + `[Cabang Aktif]`/`[Knowledge]`) + `Keterbatasan`
(`Tidak bisa ubah data`/`Asisten hanya menjawab dan merangkum, bukan mengubah
transaksi.` · `SOP butuh dokumen siap`/`Jawaban SOP dan kebijakan memakai dokumen
yang sudah selesai diproses.`).

4 kartu persis: `Ringkasan penjualan` (`Total penjualan untuk cabang aktif.`,
`Penjualan hari ini berapa?`, badge `Cabang Aktif`) · `Pesanan pending`
(`Order yang masih menunggu tindakan.`, `Order mana yang masih pending?`,
`Cabang Aktif`) · `Stok kritis` (`Produk yang perlu segera direstock.`,
`Stok apa yang menipis?`, `Cabang Aktif`) · `SOP & kebijakan`
(`Jawaban bisa memakai dokumen knowledge yang sudah siap.`,
`Bagaimana aturan retur barang?`, badge `Knowledge`). Keterbatasan 2 kartu
statis (judul + deskripsi di atas). **cek!** kartu janji `cabang aktif` padahal
BOT bisa seluruh-perusahaan bila tanpa-cabang!

## 4. Toast, Hook, Slice, Adapter

Toast (15, dari `assistant.slice.ts` — teks persis!): knowledge 4
(`Dokumen knowledge berhasil disimpan` success / `Gagal menyimpan dokumen
knowledge` danger / `Dokumen knowledge diarsipkan` warning / `Gagal mengarsipkan
dokumen` danger); otorisasi 4 (`Nomor pengakses BOT tersimpan` /
`Nomor pengakses BOT diperbarui` success + `Gagal menyimpan otorisasi WhatsApp`
danger / `Nomor pengakses BOT dihapus` warning + `Gagal mencabut akses WhatsApp`
danger); kanal 4 (`Koneksi BOT disiapkan` + info `Scan QR dari satu akun WhatsApp
perusahaan.` success / `Gagal menghubungkan kanal WhatsApp` danger /
`Nomor BOT diputuskan` warning / `Gagal memutuskan kanal WhatsApp` danger);
config 2 (`Mode asisten diperbarui` success / `Gagal memperbarui konfigurasi
asisten` danger). Semua tulis-aksi `reloadAssistantData()` sesudah-sukses;
gagal = notify + rethrow (modal tetap-terbuka untuk otorisasi/knowledge!
karena `.then(close)` tanpa-catch!).

Hook (`use-assistant-module.ts` 32 baris): muat users + refresh-data sekali
(cegah-ganda via `moduleStatus`: users bila bukan-loading/ready → `reloadUsers()`;
assistant bila loading/ready → skip else `refreshAssistantData()`); return 11
field (`activeBranch, activeWorkspaceData, archiveKnowledgeDocument, can,
connectWhatsappChannel, disconnectWhatsappChannel, findUserName,
refreshAssistantData, saveAssistantConfig, revokeWhatsappAuthorization,
saveKnowledgeDocument, saveWhatsappAuthorization`). Tanpa expose simulate/preview/
process/detail/update!

Slice (`assistant.slice.ts` 189): `reloadAssistantData` = 4 `apiPost` paralel
(`knowledge/documents/list {limit:100}`, `whatsapp/authorizations/list {}`,
`whatsapp/status {}`, `whatsapp/assistant-config/get {}`) masing-masing
`.catch(() => fallback)` (list→`{items:[]}`, status/config→`null`) + `try/catch`
luar (workspace-tetap-bisa!). Map: `toKnowledgeDocument` (fallback
`documentType:"guide"`, `status:"processing"`, tanggal-now!), `toWhatsappAuthorization`
(fallback akses/aktif!), status `{state ?? "disconnected", phone: bot_phone ??
phone ?? "", updatedAt ?? now, mode ?? "rule_based", qrDataUrl, lastErrorText}`.
Tulis: create `{title, document_type, summary, description: summary, file_name,
file_base64, tags}`; archive `{id_knowledge_document:Number(id)}`; auth
`create/update` (`{id_whatsapp_authorization:Number?, phone_number, user_name,
access_level, is_primary_owner}`); revoke sama; connect `{display_number?}`
(abaikan-server!); disconnect `{}`; config `{mode}`. Error slice ditelan saat
reload (QR bisa-spinner-selamanya bila status-gagal-terus!).

Adapter (`org.adapter.ts:116-139`): `phone = phone ?? phone_number ?? ""`
(`sender_phone` API tak-dibaca — cek! tabel tampil `phone` yang sama!);
`userName = userName ?? user_name ?? label ?? ""`; knowledge `summary =
summary ?? description ?? summary_text ?? ""`.
