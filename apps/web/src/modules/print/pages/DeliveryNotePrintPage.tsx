// Surat Jalan — A4 landscape + kertas kontinu.
//
// Dokumen yang dipegang sopir & pelanggan: isi & tata letak SAMA dengan
// delivery-note-print-page.tsx sistem lama (identitas perusahaan).
// Data: deliveryService.get + induk SO (nama barang, pelanggan, ship-to,
// pembuat order, status bayar). SO best-effort: tanpa izin baca SO surat jalan
// tetap tercetak dengan id barang. SJ Langsung tanpa order (ADR-0022): nama
// pelanggan, penerima, telepon, dan alamat dari SJ sendiri; No Nota dan
// Tanggal Nota kosong; tata letak tidak berubah (ADR-0018 D12).
//
// Multi-halaman (temuan 3): barang dipecah per 8 dan SETIAP lembar adalah dokumen lengkap — kop,
// judul + nomor, pihak, 8 slot barang (pengisi di lembar terakhir), STATUS, tanda tangan, dan
// "Halaman X dari Y" — karena lembar karbon disobek per lembar (brief desain-cetak-sj-paging).

import { useState, type ReactNode } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { Letterhead } from "@/modules/print/components/Letterhead";
import {
  formatDocDateTime,
  formatLongDate,
  printItemName,
  resolveShipToContact,
} from "@/modules/print/document";
import {
  readHideLetterhead,
  readPaperMode,
  withHideLetterheadParam,
  withPaperModeParam,
  type PrintPaperMode,
} from "@/modules/print/paper";
import { PrintProfileNotice } from "@/modules/print/components/PrintProfileNotice";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import { fetchDeliveryPrint, type DeliveryPrintData } from "@/modules/print/services/print-fetch";
import {
  deliveryItemLine,
  deliveryPartyBlock,
  deliveryStatusLabel,
  paginateDeliveryLines,
  productLinesForNote,
} from "@/modules/print/lib/delivery-document";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { continuousDeliveryStyles, deliveryStyles } from "@/modules/print/styles";

/** Catatan cetak bebas dibatasi agar muat di kaki setiap lembar (±3 baris). */
const PRINT_NOTES_MAX = 160;

// Konvensi karbon tetap (keputusan bisnis, bukan bug): dikunci agar tak terbalik.
const COPY_NOTE = "PUTIH : SOPIR   MERAH : PELANGGAN";

export default function DeliveryNotePrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, paper, corrupt, warning, loading: profileLoading } = usePrintProfile();

  // Catatan bebas sebelum cetak — hanya hasil cetak, TIDAK disimpan.
  const [printNotes, setPrintNotes] = useState("");

  const noteId = Number(id);

  const { data, loading, error } = useAsyncData(
    async (): Promise<DeliveryPrintData> => {
      if (!Number.isInteger(noteId) || noteId <= 0) {
        throw new Error("ID surat jalan tidak valid.");
      }
      try {
        return await fetchDeliveryPrint(noteId);
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat surat jalan", msg);
        throw err;
      }
    },
    [noteId],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat...</div>;
  }
  if (error !== null || data === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Surat jalan tidak ditemukan."}</div>;
  }

  const { note, order, products } = data;
  // SJ Langsung yang belum diorder: identitas pihak dari SJ, STATUS "Belum Diorder".
  const unorderedDirect = note.origin === "direct" && note.salesOrderId === 0;
  const paperMode: PrintPaperMode = readPaperMode(searchParams, paper.isDefault ? "continuous" : "a4");
  const isContinuous = paperMode === "continuous";
  const hideStoreStatic = readHideLetterhead(searchParams, !paper.showLetterhead);
  const showStoreStatic = !isContinuous || !hideStoreStatic;

  const { partyName, shipToRecipient } = deliveryPartyBlock({
    orderPartyName: order?.partyName,
    orderShipToRecipient: order?.shipToRecipient,
    notePartyName: note.partyName,
    noteShipToRecipient: note.shipToRecipient,
  });
  const contact = resolveShipToContact({
    shipToAddress: order?.shipToAddress ?? note.shipToAddress,
    shipToPhone: order?.shipToPhone ?? note.shipToPhone,
    fallbackAddress: order?.partyAddress,
    fallbackPhone: order?.partyPhone,
  });
  // Baris SO untuk nama barang; tanpa SO, master produk (best-effort).
  const nameLines = order?.items ?? (products ? productLinesForNote(note.items, products) : []);
  // Staff = pembuat order; USER = akun yang sedang mencetak.
  const staffName = order?.createdByName?.trim() || "—";
  const userName = user?.fullName?.trim() || "—";
  const statusText = deliveryStatusLabel(note.status, order?.paymentStatus, order?.paymentTerms, unorderedDirect);
  const rows = note.items.map((item) => {
    const line = deliveryItemLine(item, nameLines);
    return { item, name: line !== null ? printItemName(line) : `#${item.productId}/${item.variantId}` };
  });
  const sheets = paginateDeliveryLines(rows, { nameOf: (r) => r.name, paper: paperMode, notes: note.notes });
  const printedAt = formatDocDateTime(new Date().toISOString());

  // Bagian yang sama di setiap lembar.
  const header: ReactNode = (
    <>
      <div className="sj-head">
        {showStoreStatic ? <Letterhead identity={identity} /> : <div />}
        <div>
          <div className="sj-title">SURAT JALAN</div>
          <div className="sj-meta">{note.number}</div>
        </div>
      </div>

      <div className="sj-parties">
        <div className="sj-kv">
          <span className="lbl">Nama</span><span>:</span><span>{partyName}</span>
          {shipToRecipient !== "" && shipToRecipient !== partyName ? (
            <>
              <span className="lbl">Penerima</span><span>:</span><span>{shipToRecipient}</span>
            </>
          ) : null}
          <span className="lbl">No HP</span><span>:</span><span>{contact.phone}</span>
          <span className="lbl">Alamat</span><span>:</span><span>{contact.address}</span>
        </div>
        <div className="sj-kv">
          <span className="lbl">Tanggal Kirim</span><span>:</span><span>{formatLongDate(note.deliveryDate)}</span>
          <span className="lbl">No Nota</span><span>:</span><span>{unorderedDirect ? "" : note.orderNumber || order?.number || "-"}</span>
          <span className="lbl">Tanggal Nota</span><span>:</span><span>{unorderedDirect ? "" : formatLongDate(order?.orderDate)}</span>
          <span className="lbl">Staff</span><span>:</span><span>{staffName}</span>
          {note.vehiclePlate.trim() !== "" ? (
            <>
              <span className="lbl">Kendaraan</span><span>:</span><span>{note.vehiclePlate}</span>
            </>
          ) : null}
        </div>
      </div>
    </>
  );
  const foot: ReactNode = (
    <div className="sj-foot">
      <div style={{ whiteSpace: "pre-wrap" }}>{printNotes}</div>
      <div>
        <div className="sj-cityline">{identity.cityLine !== "" ? `${identity.cityLine}, ` : ""}................</div>
        <div className="sj-sign-grid">
          <div className="sj-sign-box">
            <div className="sj-sign-role">Petugas Gudang</div>
            <div className="sj-sign-line">{note.warehouseStaffName.trim() || "—"}</div>
          </div>
          <div className="sj-sign-box">
            <div className="sj-sign-role">Sopir</div>
            <div className="sj-sign-line">{note.driverName}</div>
          </div>
          <div className="sj-sign-box">
            <div className="sj-sign-role">Pembeli</div>
            <div className="sj-sign-line">{partyName}</div>
          </div>
        </div>
      </div>
    </div>
  );

  return (
    <PrintLayout
      baseCss={deliveryStyles()}
      continuousCss={isContinuous ? continuousDeliveryStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={{
        hidden: hideStoreStatic,
        onChange: (hidden) => setSearchParams(withHideLetterheadParam(searchParams, hidden), { replace: true }),
      }}
      notice={<PrintProfileNotice corrupt={corrupt} warning={warning} />}
      extraToolbar={
        <>
          <label className="print-toolbar-option" style={{ alignItems: "flex-start" }}>
            <span>Notes (opsional, hanya tampil di hasil cetak — tidak disimpan):</span>
            <textarea
              rows={2}
              maxLength={PRINT_NOTES_MAX}
              value={printNotes}
              onChange={(e) => setPrintNotes(e.target.value)}
              placeholder="Tulis catatan tambahan untuk surat jalan ini..."
            />
          </label>
          <span className="text-sm text-muted-foreground">{sheets.length} halaman</span>
        </>
      }
      mainClassName={isContinuous ? "sj-page continuous" : "sj-page"}
    >
      {sheets.map((sheet) => {
        const last = sheet.index === sheet.total - 1;
        return (
          <section key={sheet.index} className="sj-sheet" aria-label={`Halaman ${sheet.index + 1} dari ${sheet.total}`}>
            {header}

            <table className="doc-table sj-table">
              <colgroup>
                <col style={{ width: "4%" }} />
                <col style={{ width: "8%" }} />
                <col style={{ width: "9%" }} />
                <col />
                <col style={{ width: "26%" }} />
              </colgroup>
              <thead>
                <tr>
                  <th>No</th>
                  <th>Qty</th>
                  <th>Satuan</th>
                  <th>Produk</th>
                  <th>Keterangan</th>
                </tr>
              </thead>
              <tbody>
                {sheet.items.map(({ item, name }, idx) => (
                  <tr key={item.id}>
                    <td className="t-c">{sheet.firstNo + idx}</td>
                    <td className="t-r">{formatNumber(item.qty)}</td>
                    <td className="t-c">{item.uom}</td>
                    <td>{name}</td>
                    <td />
                  </tr>
                ))}
                {Array.from({ length: sheet.fillers }).map((_, index) => (
                  <tr key={`filler-${index}`} className="sj-filler">
                    <td>&nbsp;</td>
                    <td />
                    <td />
                    <td />
                    <td />
                  </tr>
                ))}
              </tbody>
            </table>

            <div className="sj-statusbar">
              <span className="sj-status">STATUS : {statusText}</span>
              {rows.length > 0 ? (
                <span className="sj-range">
                  Barang {sheet.firstNo}–{sheet.lastNo} dari {rows.length}
                </span>
              ) : null}
              {!last ? <span className="sj-cont">Bersambung ke halaman {sheet.index + 2}</span> : null}
            </div>
            {note.notes.trim() !== "" ? <div className="sj-note">Catatan : {note.notes}</div> : null}

            {foot}

            <div className="sj-footbar">
              <span>Dicetak: {printedAt}</span>
              <span>{COPY_NOTE}</span>
              <span>USER : {userName}</span>
              <span className="sj-pageno">
                Halaman {sheet.index + 1} dari {sheet.total}
              </span>
            </div>
          </section>
        );
      })}
    </PrintLayout>
  );
}
