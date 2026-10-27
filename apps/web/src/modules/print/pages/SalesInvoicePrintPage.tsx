// Nota / Tagihan Penjualan — A4 landscape + kertas kontinu.
//
// Dokumen yang dipegang pelanggan: isi & tata letak SAMA dengan
// sales-document-print-page.tsx sistem lama (identitas perusahaan). Angka dari
// lib/sales-document (rumus legacy), format tanggal/terbilang dari document.ts.
// Data: salesService.get (orderView membawa ship-to, kontak pelanggan, faktur
// pajak, pembuat order, ringkasan bayar).

import { useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { ShowBankToggle } from "@/modules/print/components/ShowBankToggle";
import { Letterhead } from "@/modules/print/components/Letterhead";
import {
  formatDocDateTime,
  formatLongDate,
  printItemName,
  resolveShipToContact,
  terbilangRupiah,
  transferToListLine,
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
import { fetchSalesOrderForPrint, type SalesOrder } from "@/modules/print/services/print-fetch";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import {
  documentTotals,
  dueDateCell,
  effectiveHideDiscount,
  hasLineDiscount,
  HIDE_DISCOUNT_UNPAID_NOTE,
  hideDiscountAllowed,
  lineDiscountTotal,
  linePrintUnitPrice,
  lineSellingAmount,
  orderPrintDiscount,
  readHideDiscount,
  salesDocumentTitle,
  withHideDiscountParam,
} from "@/modules/print/lib/sales-document";
import { continuousInvoiceStyles, invoiceStyles } from "@/modules/print/styles";

/** Baris minimal tabel nota A4 (form tampak penuh); kertas kontinu tanpa pengisi. */
const NOTA_MIN_ROWS = 6;

export default function SalesInvoicePrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, paper, corrupt, warning, loading: profileLoading } = usePrintProfile();

  const orderId = Number(id);
  // Catatan bebas sebelum cetak — hanya tercetak, tidak pernah disimpan ke order.
  const [printNotes, setPrintNotes] = useState("");
  // Rekening hanya tercetak bila dicentang sebelum cetak (bawaan tidak).
  const [showBank, setShowBank] = useState(false);

  const { data: order, loading, error } = useAsyncData(
    async (): Promise<SalesOrder> => {
      if (!Number.isInteger(orderId) || orderId <= 0) {
        throw new Error("ID order tidak valid.");
      }
      try {
        return await fetchSalesOrderForPrint(orderId);
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat nota", msg);
        throw err;
      }
    },
    [orderId],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat dokumen…</div>;
  }
  if (error !== null || order === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Order tidak ditemukan."}</div>;
  }

  const paperMode: PrintPaperMode = readPaperMode(searchParams, paper.isDefault ? "continuous" : "a4");
  const isContinuous = paperMode === "continuous";
  // Kertas kontinu: JANGAN isi baris kosong — tinggi form fisik (5,5") pas-pasan.
  const fillerRows = isContinuous ? 0 : Math.max(0, NOTA_MIN_ROWS - order.items.length);
  // Kop/daftar jual disembunyikan bila form kontinu sudah preprinted (default dari
  // Pengaturan, bisa ditimpa per cetak).
  const hideStoreStatic = readHideLetterhead(searchParams, !paper.showLetterhead);
  const showStoreStatic = !isContinuous || !hideStoreStatic;
  // Sembunyikan diskon hanya untuk nota yang sudah lunas (ADR-0018 D12).
  const hideAllowed = hideDiscountAllowed(order);
  const hideDiscount = effectiveHideDiscount(readHideDiscount(searchParams), order);
  const printDiscount = orderPrintDiscount(order);
  const totals = documentTotals(order, hideDiscount);
  // "Transfer ke: …" = daftar rekening dokumen, hanya untuk tagihan yang masih bersisa (ADR-0026).
  const transferLine = totals.outstanding > 0 ? transferToListLine(identity.bankAccounts) : "";
  // Kolom Disc hanya bila ada baris yang punya diskonnya sendiri.
  const showDiscColumn = hasLineDiscount(order.items) && !hideDiscount;
  const dueDate = dueDateCell(order);
  const contact = resolveShipToContact({
    shipToAddress: order.shipToAddress,
    shipToPhone: order.shipToPhone,
    fallbackAddress: order.partyAddress,
    fallbackPhone: order.partyPhone,
  });
  const customerName = order.partyId > 0 && order.partyName.trim() !== "" ? order.partyName : "Pelanggan umum";
  // Staff = pembuat order; USER = akun yang sedang mencetak.
  const staffName = order.createdByName?.trim() || "-";
  const userName = user?.fullName?.trim() || "-";

  return (
    <PrintLayout
      baseCss={invoiceStyles()}
      continuousCss={isContinuous ? continuousInvoiceStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={{
        hidden: hideStoreStatic,
        onChange: (hidden) => setSearchParams(withHideLetterheadParam(searchParams, hidden), { replace: true }),
      }}
      notice={<PrintProfileNotice corrupt={corrupt} warning={warning} />}
      mainClassName={isContinuous ? "nota-page continuous" : "nota-page"}
      extraToolbar={
        <>
          {printDiscount > 0 ? (
            <label className="print-toolbar-option" title={hideAllowed ? undefined : HIDE_DISCOUNT_UNPAID_NOTE}>
              <input
                type="checkbox"
                checked={hideDiscount}
                disabled={!hideAllowed}
                aria-describedby={hideAllowed ? undefined : "hide-discount-note"}
                onChange={(e) => setSearchParams(withHideDiscountParam(searchParams, e.target.checked), { replace: true })}
              />
              <span>Sembunyikan diskon (harga penuh)</span>
              {hideAllowed ? null : (
                <span id="hide-discount-note" style={{ fontWeight: 400, color: "#92400e" }}>
                  — hanya untuk nota lunas. {HIDE_DISCOUNT_UNPAID_NOTE}.
                </span>
              )}
            </label>
          ) : null}
          {transferLine !== "" ? <ShowBankToggle checked={showBank} onChange={setShowBank} /> : null}
          <label className="print-toolbar-option" style={{ alignItems: "flex-start" }}>
            <span>Notes (opsional, hanya tampil di hasil cetak — tidak disimpan):</span>
            <textarea
              rows={2}
              value={printNotes}
              onChange={(e) => setPrintNotes(e.target.value)}
              placeholder="Tulis catatan tambahan untuk nota ini..."
            />
          </label>
        </>
      }
    >
      <div className="nota-head">
        {showStoreStatic ? <Letterhead identity={identity} /> : <div />}
        <div className="nota-title">{salesDocumentTitle(totals, order.status)}</div>
      </div>

      <div className="nota-parties">
        <div className="nota-kv">
          <span className="lbl">Nama</span><span>:</span><span>{customerName}</span>
          <span className="lbl">No HP</span><span>:</span><span>{contact.phone}</span>
          <span className="lbl">Alamat</span><span>:</span><span>{contact.address}</span>
        </div>
        <div className="nota-kv">
          <span className="lbl">No Nota</span><span>:</span><span>{order.number}</span>
          <span className="lbl">Tanggal</span><span>:</span><span>{formatLongDate(order.orderDate)}</span>
          <span className="lbl">Tgl. Jtp</span><span>:</span><span>{dueDate !== null ? formatLongDate(dueDate) : "-"}</span>
          <span className="lbl">Staff</span><span>:</span><span>{staffName}</span>
          {order.taxInvoiceNumber.trim() !== "" ? (
            <>
              <span className="lbl">Faktur Pajak</span><span>:</span><span>{order.taxInvoiceNumber}</span>
            </>
          ) : null}
        </div>
      </div>

      <table className="doc-table nota-table">
        <colgroup>
          <col style={{ width: "4%" }} />
          <col style={{ width: "7%" }} />
          <col style={{ width: "9%" }} />
          <col />
          <col style={{ width: "13%" }} />
          {showDiscColumn ? <col style={{ width: "10%" }} /> : null}
          <col style={{ width: "13%" }} />
        </colgroup>
        <thead>
          <tr>
            <th>No</th>
            <th>Qty</th>
            <th>Satuan</th>
            <th>Produk</th>
            <th>Harga Satuan</th>
            {showDiscColumn ? <th>Disc</th> : null}
            <th>Harga Jual</th>
          </tr>
        </thead>
        <tbody>
          {order.items.map((item, idx) => (
            <tr key={item.id}>
              <td className="t-c">{idx + 1}</td>
              <td className="t-r">{formatNumber(item.qty)}</td>
              <td className="t-c">{item.uom}</td>
              <td>
                {printItemName(item)}
                {item.note.trim() !== "" ? <div style={{ fontSize: "8pt", color: "#555" }}>{item.note}</div> : null}
              </td>
              <td className="t-r">{formatIDR(linePrintUnitPrice(item))}</td>
              {showDiscColumn ? <td className="t-r">{formatIDR(lineDiscountTotal(item))}</td> : null}
              <td className="t-r">{formatIDR(lineSellingAmount(item, hideDiscount))}</td>
            </tr>
          ))}
          {Array.from({ length: fillerRows }).map((_, index) => (
            <tr key={`filler-${index}`}>
              <td>&nbsp;</td>
              <td />
              <td />
              <td />
              <td />
              {showDiscColumn ? <td /> : null}
              <td />
            </tr>
          ))}
        </tbody>
      </table>

      <div className="nota-sumbar">
        <span className="nota-terbilang">terbilang : {terbilangRupiah(totals.total)}</span>
        <table className="nota-paysum">
          <tbody>
            <tr><td>Jumlah (Rp)</td><td>{formatIDR(totals.grossSubtotal)}</td></tr>
            {totals.discount !== null ? <tr><td>Diskon</td><td>{formatIDR(totals.discount)}</td></tr> : null}
            {totals.tax !== null ? <tr><td>Pajak</td><td>{formatIDR(totals.tax)}</td></tr> : null}
            <tr><td>Total</td><td>{formatIDR(totals.total)}</td></tr>
            {totals.paid !== null ? <tr><td>Sudah Dibayar</td><td>{formatIDR(totals.paid)}</td></tr> : null}
            <tr className="bold"><td>Sisa Tagihan</td><td>{formatIDR(totals.outstanding)}</td></tr>
          </tbody>
        </table>
      </div>

      <div className="nota-foot">
        <div>
          {/* Rekening perusahaan (ADR-0026): satu baris di kaki, tetap tercetak di form preprinted. */}
          {showBank && transferLine !== "" ? (
            <div style={{ fontWeight: 700 }}>{transferLine}</div>
          ) : null}
          <div style={{ whiteSpace: "pre-wrap" }}>{printNotes}</div>
        </div>
        <div className="nota-buyer">
          <div>Pembeli</div>
          <div className="nota-buyer-line">(................)</div>
          {identity.returnNote !== "" ? <div style={{ fontSize: "8pt" }}>{identity.returnNote}</div> : null}
        </div>
        <div className="nota-sell">
          {showStoreStatic && identity.sellingPoints.length > 0 ? (
            <>
              <div className="nota-sell-title">{identity.name.toUpperCase()} MENJUAL:</div>
              <ul>
                {identity.sellingPoints.map((point, index) => (
                  <li key={`sell-${index}`}>{point}</li>
                ))}
              </ul>
            </>
          ) : null}
        </div>
      </div>

      <div className="nota-footbar">
        <span>Dicetak: {formatDocDateTime(new Date().toISOString())}</span>
        <span>USER : {userName}</span>
      </div>
    </PrintLayout>
  );
}
