// Bukti Pembayaran — A4 + kertas kontinu.
//
// Dokumen yang dipegang pelanggan/pemasok: isi & tata letak SAMA dengan
// payment-receipt-print-page.tsx sistem lama (identitas perusahaan), tanpa
// blok tanda tangan (BR-30). Pembayaran v2 bisa dialokasi ke beberapa order
// (tidak ada di sistem lama): satu order -> ringkasan tagihan legacy; beberapa
// order -> tabel alokasi menggantikan ringkasan itu.

import { useParams, useSearchParams } from "react-router-dom";
import { formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { formatDocDate, formatDocDateTime, printMethodLabel, transferToLine } from "@/modules/print/document";
import { readPaperMode, withPaperModeParam, type PrintPaperMode } from "@/modules/print/paper";
import { PrintProfileNotice } from "@/modules/print/components/PrintProfileNotice";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import {
  fetchAllocationBilling,
  fetchPaymentReceipt,
  type AllocationBilling,
  type PaymentReceiptData,
} from "@/modules/print/services/print-fetch";
import { receiptOrderRef, receiptPartySide, receiptSummary } from "@/modules/print/lib/payment-receipt";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { orderTypeLabel } from "@/modules/payment/types";
import { continuousReceiptStyles, receiptStyles } from "@/modules/print/styles";

export default function PaymentReceiptPrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const company = useAuthStore((s) => s.company);
  const branch = useAuthStore((s) => s.branch);
  const { paper, corrupt, warning, loading: profileLoading, bankAccounts } = usePrintProfile();

  const paymentId = Number(id);

  const { data, loading, error } = useAsyncData(
    async (): Promise<PaymentReceiptData & { billing: Record<string, AllocationBilling> }> => {
      if (!Number.isInteger(paymentId) || paymentId <= 0) {
        throw new Error("ID pembayaran tidak valid.");
      }
      try {
        const receipt = await fetchPaymentReceipt(paymentId);
        // Tabel alokasi (multi order) — ringkasan tagihan per order, best-effort.
        const billing =
          receipt.payment.allocations.length > 1 ? await fetchAllocationBilling(receipt.payment.allocations) : {};
        return { ...receipt, billing };
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat bukti pembayaran", msg);
        throw err;
      }
    },
    [paymentId],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat bukti pembayaran...</div>;
  }
  if (error !== null || data === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Pembayaran tidak ditemukan."}</div>;
  }

  const { payment, order, history, billing } = data;
  const paperMode: PrintPaperMode = readPaperMode(searchParams, paper.isDefault ? "continuous" : "a4");
  const isContinuous = paperMode === "continuous";

  const ref = receiptOrderRef(payment);
  const side = receiptPartySide(ref?.orderType, payment.partyType, payment.direction);
  const isCustomer = side === "customer";
  const summary = ref !== null && order !== null ? receiptSummary(payment, ref, order.total, history) : null;
  const cancelled = payment.status === "cancelled";
  const statusText = cancelled ? "Dibatalkan" : summary !== null ? (summary.outstandingAfter <= 0 ? "Lunas" : "Ada Sisa") : null;
  const partyName =
    payment.partyId > 0 && payment.partyName.trim() !== ""
      ? payment.partyName
      : isCustomer
        ? "Pelanggan umum"
        : "Supplier";
  const orderNumber =
    order?.number ?? (payment.allocations.length > 0 ? payment.allocations.map((a) => a.orderNumber).join(", ") : "-");
  const companyName = company?.name?.trim() || "Mini ERP";

  return (
    <PrintLayout
      baseCss={receiptStyles()}
      continuousCss={isContinuous ? continuousReceiptStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      // Kepala bukti pembayaran berupa teks (tanpa kop bergambar): tak ada toggle preprinted.
      letterheadToggle={null}
      notice={<PrintProfileNotice corrupt={corrupt} warning={warning} />}
      mainClassName={isContinuous ? "kwitansi-page continuous" : "kwitansi-page"}
    >
      <div className="kwitansi-head">
        <div>
          <h1 className="kwitansi-title">{isCustomer ? "Bukti Pembayaran Customer" : "Bukti Pembayaran Supplier"}</h1>
          <div className="kwitansi-company">{companyName}</div>
          {branch !== null ? (
            <div className="kwitansi-muted">
              {branch.name}
              {branch.address?.trim() ? ` - ${branch.address.trim()}` : ""}
            </div>
          ) : null}
        </div>
        <div className="kwitansi-meta">
          <div><strong>No. Dokumen:</strong> {payment.number}</div>
          <div><strong>Tanggal:</strong> {formatDocDate(payment.paidAt)}</div>
          {statusText !== null ? (
            <div><span className="kwitansi-status">{statusText}</span></div>
          ) : null}
        </div>
      </div>

      <section className="kwitansi-grid">
        <div className="kwitansi-box">
          <h2>{isCustomer ? "Diterima Dari" : "Dibayarkan Kepada"}</h2>
          <div className="kwitansi-kv">
            <span>Nama</span><strong>{partyName}</strong>
            <span>No. HP</span><span>{payment.partyPhone?.trim() || "-"}</span>
            <span>Alamat</span><span>{payment.partyAddress?.trim() || "-"}</span>
          </div>
        </div>
        <div className="kwitansi-box">
          <h2>Rincian Pembayaran</h2>
          <div className="kwitansi-kv">
            <span>No. Order</span><strong>{orderNumber}</strong>
            <span>Tanggal</span><span>{formatDocDateTime(payment.paidAt)}</span>
            <span>Metode</span><span>{printMethodLabel(payment.method)}</span>
            {transferToLine(bankAccounts, payment.cashAccountId) !== "" ? (
              <>
                <span>{payment.direction === "out" ? "Dari rekening" : "Ke rekening"}</span>
                <span>{transferToLine(bankAccounts, payment.cashAccountId, "")}</span>
              </>
            ) : null}
            <span>Referensi</span><span>{payment.reference.trim() || "-"}</span>
          </div>
        </div>
      </section>

      {payment.allocations.length > 1 ? (
        <table className="kwitansi-alloc">
          <thead>
            <tr>
              <th>No. Order</th>
              <th>Jenis</th>
              <th style={{ textAlign: "right" }}>Nominal</th>
              <th style={{ textAlign: "right" }}>Total Tagihan</th>
              <th style={{ textAlign: "right" }}>Sisa Tagihan (saat cetak)</th>
            </tr>
          </thead>
          <tbody>
            {payment.allocations.map((alloc, idx) => {
              const b = billing[`${alloc.orderType}:${alloc.orderId}`];
              return (
                <tr key={`${alloc.orderType}-${alloc.orderId}-${idx}`}>
                  <td>{alloc.orderNumber}</td>
                  <td>{orderTypeLabel(alloc.orderType)}</td>
                  <td className="num">{formatIDR(alloc.amount)}</td>
                  <td className="num">{b ? formatIDR(b.total) : "-"}</td>
                  <td className="num">{b ? formatIDR(b.outstanding) : "-"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      ) : null}

      <table className="kwitansi-summary">
        <tbody>
          {summary !== null ? (
            <tr><td>Total Tagihan</td><td className="num">{formatIDR(summary.totalAmount)}</td></tr>
          ) : null}
          <tr className="grand-total"><td>Jumlah Pembayaran Ini</td><td className="num">{formatIDR(payment.amount)}</td></tr>
          {summary !== null ? (
            <>
              <tr><td>Total Terbayar Setelah Ini</td><td className="num">{formatIDR(summary.paidUntilThis)}</td></tr>
              <tr className="grand-total"><td>Sisa Setelah Pembayaran</td><td className="num">{formatIDR(summary.outstandingAfter)}</td></tr>
            </>
          ) : null}
        </tbody>
      </table>

      {payment.notes.trim() !== "" ? (
        <div className="kwitansi-note">
          <strong>Catatan:</strong> {payment.notes}
        </div>
      ) : null}

      <div className="kwitansi-note">
        Bukti pembayaran ini dicatat dari sistem dan diverifikasi melalui nomor pembayaran, metode, referensi, serta audit transaksi.
      </div>
    </PrintLayout>
  );
}
