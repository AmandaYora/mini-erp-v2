# Algorithms Legacy — Modul 06 Business Party (Customer / Supplier)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**, bukan cara implementasi.

---

## A-01 — Kode otomatis anti-pakai-ulang (hasil: kosongkan = dapat kode unik selamanya)

**Hasil yang diharapkan:** kode kosong/whitespace selalu menghasilkan `CUS-###`/`SUP-###` unik yang
tidak pernah dipakai ulang bahkan setelah arsip (suffix = max+1 lintas arsip, tumbuh melewati 999,
manual berpola ikut menggeser, manual tak berpola diabaikan). Duplikat manual ditolak dengan pesan
yang menyebut Sampah bila pelakunya arsip. **Bebas diubah:** query max, padding, dengan catatan
konkuren perlu kunci/retry (kondisi kini: bisa 500).

## A-02 — Arsip relasional (hasil: disembunyikan tanpa diputus)

**Hasil yang diharapkan:** arsip menghilangkan pihak dari daftar/picker/order-baru, tetapi saldo,
ledger, pelunasan, order lama, dan nama-live tetap berfungsi. Restore menghidupkan kembali bila tak
bentrok kode. Alamat mengikuti filosofi snapshot: dokumen lama stabil walau buku berubah/diarsip.
**Bebas diubah:** nilai flag, endpoint, selama sifat "sembunyi-tanpa-putus" bertahan — ini pembeda
modul ini dari produk (tanpa restore) dan dari hapus-permanen role.

## A-03 — Primer tunggal buku alamat (hasil: selalu ada default selagi ada alamat)

**Hasil yang diharapkan:** tepat satu Utama selama ≥1 alamat aktif: pertama otomatis, penunjukan
memindahkan dalam satu transaksi, arsip Utama mempromosikan pengganti paling awal, kekosongan total
diizinkan. Daftar selalu primer-dulu. **Bebas diubah:** mekanisme transaksi, dengan catatan
pertimbangkan constraint parsial di DB agar invariant tidak hanya di aplikasi.

## A-04 — Pencarian identitas frasa (hasil: satu kotak menemukan siapa pun)

**Hasil yang diharapkan:** satu ketikan mencari di nama/kode/telepon/email/alamat sekaligus; dipakai
daftar dan picker serta pencarian order lintas identitas. **Bebas diubah:** implementasi LIKE,
dengan catatan putuskan tokenized vs frasa (kini frasa — beda dengan produk; → KI-65).

## A-05 — Anomali 100-baris dan perbaikannya (hasil: data ke-N tetap bisa diedit/dipilih)

**Hasil yang diharapkan (terkunci E2E 21):** edit selalu by-id (tidak pernah jatuh ke tambah),
picker selalu server-side (tidak bergantung store 100-baris), shortcut selalu me-seed pilihan.
Insiden "edit malah menambah" dan "data tidak masuk" tidak boleh terulang. **Wajib dipertahankan**
sebagai regresi, bukan sekadar fitur.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Auto-code unik lintas arsip + pesan Sampah (A-01) | Query max, lock/retry konkuren |
| Arsip sembunyi-tanpa-putus + restore berbentrok-aman (A-02) | Flag, endpoint, pesan |
| Satu Utama + promosi otomatis (A-03) | Transaksi / constraint parsial |
| Satu kotak cari lima kolom (A-04) | LIKE / FTS / tokenized (putuskan) |
| By-id + server-side + seed (A-05) | Store limit, resolve |
| Snapshot ship-to stabil (BR-31) | Kolom snapshot |
| Tiga pintu satu entitas + kode unik lintas pintu (UF-02) | Modal/panel masing-masing |
