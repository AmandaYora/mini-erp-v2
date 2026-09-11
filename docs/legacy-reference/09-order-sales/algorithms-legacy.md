# Algorithms Legacy — Modul 09 Order Sales

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**. Mekanika bersama (nomor, rupiah, tempo-net, guard, agregat) =
modul 08.

---

## A-01 — Harga jual berlapis (hasil: kasir tak bisa jual di bawah modal kebijakan)

**Hasil yang diharapkan:** saran jual → member (bila ada, boleh di bawah minimum) → diskon
terklem-minimum → tolak bersih-di-bawah-minimum; edit hormat-manual; requote klem-ulang.
**Bebas diubah:** titik klem, selama contoh (1000/3→333; member-bypass) tetap.

## A-02 — Serah sekali-tembak (hasil: stok keluar = selesai, tanpa sisa menggantung)

**Hasil yang diharapkan:** satu panggilan menghabiskan seluruh baris tracked (alokasi +
reservasi), tanggal prioritas-3, selesai + history otomatis, COD menuntut keputusan,
tunai-kurang ditolak, ganda ditolak. **Bebas diubah:** alokasi, selama sekali-tuntas bertahan
(beda dengan terima-parsial purchase).

## A-03 — Tempo sebagai persetujuan (hasil: utang selalu bertanggal + berpenanda)

**Hasil yang diharapkan:** non-net → net + tempo + approve/oleh/waktu + audit; dialog default
+30 hari; tanpa tanggal ditolak di semua lapis. **Bebas diubah:** widget, default hari.

## A-04 — Nota jujur dua-mode (hasil: yang dicetak = yang ditagih, apa pun modenya)

**Hasil yang diharapkan:** harga-penuh + bruto + sisa-riil di kedua mode; judul = status bayar;
tombol = kebalikannya; Staff pembuat + USER pencetak; `Tgl. Jtp` `-` bila lunas; ganjal A4-6.
**Bebas diubah:** CSS cetak, selama rumus tampil bertahan.

## A-05 — Uang bulat di pintu kasir (hasil: tak ada sen masuk lewat form)

**Hasil yang diharapkan:** pecahan dibuang (bukan digabung); tunai-kurang ditolak; kembalian
tampil-tak-tersimpan; referensi auto-bisa-ubah. **Bebas diubah:** parsing input.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Lapis harga + bypass-member + contoh (A-01) | Klem, quote |
| Sekali-tuntas + keputusan-COD + prioritas-tanggal (A-02) | Alokasi, reservasi |
| Tempo + approve + audit (A-03) | Dialog, default |
| Penuh + bruto + sisa-riil + judul-bayar (A-04) | CSS, param URL |
| Bulat + tolak-kurang + tampil-kembalian (A-05) | Parse, tender |
