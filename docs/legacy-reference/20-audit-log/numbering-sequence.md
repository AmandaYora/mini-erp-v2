# Numbering & Sequence — Modul 20 Audit Log

**Kelompok A. Tidak ada penomoran dokumen di modul ini** (verifikasi: tanpa
sequence/counter/format nomor di `apps/api/src/modules/audit-log/`).

ID baris = auto-increment DB (`id_audit_log`) — teknis, bukan nomor dokumen;
tidak ditampilkan di UI (kunci baris tabel = id internal).

Bukan penomoran modul ini (jangan tertukar saat rebuild): nomor-nomor yang muncul di
kolom `after_json` (mis. `orderNumber`, `paymentNumber`, `journalNumber`,
`sjNumber`, `transferNumber`, `returnNumber`, `expenseNumber`) adalah nomor DOKUMEN
modul asal — audit hanya menyalinnya sebagai ringkasan, bukan menerbitkannya.
