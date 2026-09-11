// Surat Jalan — A4 landscape + kertas kontinu (Tahap E2).
//
// Data: deliveryService.get + induk SO best-effort (nama produk/pelanggan).
// Kolom C1 (supir/plat/petugas/penerima) dan ship-to C2 ikut tercetak.

import { useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { formatDate, formatDateTime, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { BankAccountsBlock, Letterhead } from "@/modules/print/components/Letterhead";
import { resolveShipToContact } from "@/modules/print/document";
import {
  readHideLetterhead,
  readPaperMode,
  withHideLetterheadParam,
  withPaperModeParam,
  type PrintPaperMode,
} from "@/modules/print/paper";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import {
  fetchDeliveryPrint,
  type DeliveryPrintData,
} from "@/modules/print/services/print-fetch";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import {
  continuousDeliveryStyles,
  deliveryStyles,
} from "@/modules/print/styles";

const COPY_NOTE = "PUTIH : SOPIR   MERAH : PELANGGAN";

function textOrDash(v: string | null | undefined): string {
  return v !== null && v !== undefined && v.trim() !== "" ? v : "-";
}

export default function DeliveryNotePrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, paper, corrupt, loading: profileLoading } = usePrintProfile();

  // Catatan bebas sebelum cetak — hanya hasil cetak, TIDAK disimpan ke API.
  const [printNotes, setPrintNotes] = useState("");

  const noteId = Number(id);

  const {
    data,
    loading,
    error,
  } = useAsyncData(
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
    return <div style={{ padding: "2rem" }}>Memuat surat jalan…</div>;
  }
  if (error !== null || data === null) {
    return (
      <div style={{ padding: "2rem" }}>{error ?? "Surat jalan tidak ditemukan."}</div>
    );
  }

  const { note, order } = data;
  const paperMode: PrintPaperMode = readPaperMode(
    searchParams,
    paper.isDefault ? "continuous" : "a4",
  );
  const isContinuous = paperMode === "continuous";
  // Form kontinu pendek (5,5") — tidak perlu baris pengisi ala A4.
  const fillerRows = isContinuous ? 0 : Math.max(0, 5 - note.items.length);
  const hideLetterhead = readHideLetterhead(searchParams, !paper.showLetterhead);
  const showStatic = !isContinuous || !hideLetterhead;

  const partyName = order?.partyName ?? "-";
  const shipToRecipient = (order?.shipToRecipient ?? "").trim();
  const contact = resolveShipToContact({
    shipToAddress: order?.shipToAddress,
    shipToPhone: order?.shipToPhone,
    fallbackAddress: null,
    fallbackPhone: null,
  });
  const staffLine = [
    note.warehouseStaffName.trim() !== "" ? note.warehouseStaffName : null,
    note.driverName.trim() !== "" ? `Sopir: ${note.driverName}` : null,
    note.vehiclePlate.trim() !== "" ? `(${note.vehiclePlate})` : null,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <PrintLayout
      baseCss={deliveryStyles()}
      continuousCss={isContinuous ? continuousDeliveryStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={{
        hidden: hideLetterhead,
        onChange: (hidden) =>
          setSearchParams(withHideLetterheadParam(searchParams, hidden), { replace: true }),
      }}
      notice={
        corrupt ? (
          <div className="no-print print-warning">
            Profil kertas tersimpan rusak — memakai default 241,3 × 139,7 mm. Perbaiki di kalibrasi kertas.
          </div>
        ) : undefined
      }
      extraToolbar={
        <label className="print-toolbar-option" style={{ alignItems: "flex-start" }}>
          <span>Notes (opsional, hanya di hasil cetak):</span>
          <textarea
            rows={2}
            value={printNotes}
            onChange={(e) => setPrintNotes(e.target.value)}
            placeholder="Catatan tambahan…"
          />
        </label>
      }
      mainClassName={isContinuous ? "sj-page continuous" : "sj-page"}
    >
      <div className="sj-head">
        {showStatic ? <Letterhead identity={identity} /> : <div />}
        <div>
          <div className="sj-title">Surat Jalan</div>
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
          <span className="lbl">Tanggal Kirim</span><span>:</span><span>{formatDate(note.deliveryDate)}</span>
          <span className="lbl">No Nota</span><span>:</span><span>{note.orderNumber || `#${note.salesOrderId}`}</span>
          {order !== null ? (
            <>
              <span className="lbl">Tanggal Nota</span><span>:</span><span>{formatDate(order.orderDate)}</span>
            </>
          ) : null}
          <span className="lbl">Supir</span><span>:</span><span>{textOrDash(note.driverName)}</span>
          <span className="lbl">Kendaraan</span><span>:</span><span>{textOrDash(note.vehiclePlate)}</span>
        </div>
      </div>

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
          {note.items.map((item, idx) => {
            const match =
              order?.items.find(
                (l) => l.productId === item.productId && l.variantId === item.variantId,
              ) ?? null;
            return (
              <tr key={item.id}>
                <td className="t-c">{idx + 1}</td>
                <td className="t-r">{formatNumber(item.qty)}</td>
                <td className="t-c">{item.uom}</td>
                <td>
                  {match !== null ? (
                    <>
                      <span style={{ fontWeight: 600 }}>{match.productName}</span>{" "}
                      <span style={{ fontSize: "0.85em", color: "#4b5563" }}>{match.productCode}</span>
                    </>
                  ) : (
                    <span style={{ color: "#6b7280" }}>#{item.productId}/{item.variantId}</span>
                  )}
                </td>
                <td />
              </tr>
            );
          })}
          {Array.from({ length: fillerRows }).map((_, index) => (
            <tr key={`filler-${index}`}>
              <td>&nbsp;</td>
              <td />
              <td />
              <td />
              <td />
            </tr>
          ))}
        </tbody>
      </table>

      <div className="sj-status">
        STATUS : {note.status === "confirmed" ? "Terkirim" : note.status === "cancelled" ? "Dibatalkan" : "Draf"}
        {note.recipientName.trim() !== "" ? ` · Diterima: ${note.recipientName}` : ""}
      </div>
      {note.dropLocationNote.trim() !== "" ? (
        <div className="sj-note">Titik turun: {note.dropLocationNote}</div>
      ) : null}
      {note.notes.trim() !== "" ? (
        <div className="sj-note">Catatan: {note.notes}</div>
      ) : null}

      <div className="sj-foot">
        <div style={{ whiteSpace: "pre-wrap" }}>{printNotes}</div>
        <div>
          <div className="sj-cityline">
            {identity.cityLine !== "" ? `${identity.cityLine}, ` : ""}................
          </div>
          <div className="sj-sign-grid">
            <div className="sj-sign-box">
              <div className="sj-sign-role">Petugas Gudang</div>
              <div className="sj-sign-line">{textOrDash(note.warehouseStaffName)}</div>
            </div>
            <div className="sj-sign-box">
              <div className="sj-sign-role">Sopir</div>
              <div className="sj-sign-line">{textOrDash(note.driverName)}</div>
            </div>
            <div className="sj-sign-box">
              <div className="sj-sign-role">Pembeli</div>
              <div className="sj-sign-line">{textOrDash(note.recipientName || partyName)}</div>
            </div>
          </div>
        </div>
      </div>

      {showStatic && identity.bankAccounts.length > 0 ? (
        <div style={{ marginTop: "10px" }}>
          <BankAccountsBlock accounts={identity.bankAccounts} />
        </div>
      ) : null}

      <div className="sj-footbar">
        <span>Dicetak: {formatDateTime(new Date().toISOString())}</span>
        <span>{COPY_NOTE}</span>
        <span>USER : {user?.fullName?.trim() !== "" ? user?.fullName : "-"}</span>
      </div>
      {staffLine !== "" ? (
        <div className="sj-footbar">
          <span>{staffLine}</span>
        </div>
      ) : null}
    </PrintLayout>
  );
}
