// Kwitansi Pembayaran — A4 + kertas kontinu (Tahap E4).
//
// Data: paymentService.get (paymentView + alokasi order). Nominal + terbilang
// + alokasi per order tercetak. Paritas legacy (BR-30): kwitansi TANPA blok
// tanda tangan (disengaja); terbilang dipertahankan.

import { useParams, useSearchParams } from "react-router-dom";
import { formatDate, formatDateTime, formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { terbilangRupiah } from "@/modules/print/document";
import {
  readPaperMode,
  withPaperModeParam,
  type PrintPaperMode,
} from "@/modules/print/paper";
import { PrintProfileNotice } from "@/modules/print/components/PrintProfileNotice";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import {
  fetchAllocationBilling,
  fetchPaymentForPrint,
  type AllocationBilling,
  type Payment,
} from "@/modules/print/services/print-fetch";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { directionLabel, methodLabel, orderTypeLabel } from "@/modules/payment/types";
import { continuousReceiptStyles, receiptStyles } from "@/modules/print/styles";

export default function PaymentReceiptPrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const { identity, paper, corrupt, warning, loading: profileLoading } = usePrintProfile();

  const paymentId = Number(id);

  const { data: bundle, loading, error } = useAsyncData(
    async (): Promise<{ payment: Payment; billing: Record<string, AllocationBilling> }> => {
      if (!Number.isInteger(paymentId) || paymentId <= 0) {
        throw new Error("ID pembayaran tidak valid.");
      }
      try {
        const p = await fetchPaymentForPrint(paymentId);
        // Ringkasan tagihan per order dialokasi: best-effort, tak menggagalkan cetak.
        return { payment: p, billing: await fetchAllocationBilling(p.allocations) };
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat kwitansi", msg);
        throw err;
      }
    },
    [paymentId],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat kwitansi…</div>;
  }
  const payment: Payment | null = bundle?.payment ?? null;
  const billing = bundle?.billing ?? {};
  if (error !== null || payment === null) {
    return (
      <div style={{ padding: "2rem" }}>{error ?? "Pembayaran tidak ditemukan."}</div>
    );
  }

  const paperMode: PrintPaperMode = readPaperMode(
    searchParams,
    paper.isDefault ? "continuous" : "a4",
  );
  const isContinuous = paperMode === "continuous";
  // Kwitansi memakai header class-based (kop perusahaan teks) — tidak ada
  // blok kop bergambar, jadi toggle preprinted tidak ditawarkan di sini.

  const incoming = payment.direction.toLowerCase() === "in";
  const title = incoming ? "Kwitansi Penerimaan" : "Kwitansi Pembayaran";
  const partyRole = incoming ? "Diterima Dari" : "Dibayarkan Kepada";
  const partyName = payment.partyName.trim() !== "" ? payment.partyName : "-";

  return (
    <PrintLayout
      baseCss={receiptStyles()}
      continuousCss={isContinuous ? continuousReceiptStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={null}
      notice={<PrintProfileNotice corrupt={corrupt} warning={warning} />}
      mainClassName={isContinuous ? "kwitansi-page continuous" : "kwitansi-page"}
    >
      <div className="kwitansi-head">
        <div>
          <h1 className="kwitansi-title">{title}</h1>
          <div className="kwitansi-company">{identity.name}</div>
          {identity.addressLine !== "" ? (
            <div className="kwitansi-muted">{identity.addressLine}</div>
          ) : null}
        </div>
        <div className="kwitansi-meta">
          <div><strong>No. Dokumen:</strong> {payment.number}</div>
          <div><strong>Tanggal:</strong> {formatDate(payment.paidAt)}</div>
          <div>
            <span className="kwitansi-status">
              {payment.status === "cancelled" ? "Dibatalkan" : directionLabel(payment.direction)}
            </span>
          </div>
        </div>
      </div>

      <section className="kwitansi-grid">
        <div className="kwitansi-box">
          <h2>{partyRole}</h2>
          <div className="kwitansi-kv">
            <span>Nama</span><strong>{partyName}</strong>
            <span>Jenis</span><span>{payment.partyType || "-"}</span>
          </div>
        </div>
        <div className="kwitansi-box">
          <h2>Rincian Pembayaran</h2>
          <div className="kwitansi-kv">
            <span>Metode</span><span>{methodLabel(payment.method)}</span>
            <span>Tanggal</span><span>{formatDateTime(payment.paidAt)}</span>
            {payment.method === "cash" && payment.amountTendered > 0 ? (
              <>
                <span>Tunai diterima</span><span>{formatIDR(payment.amountTendered)}</span>
                <span>Kembalian</span><span>{formatIDR(payment.change)}</span>
              </>
            ) : null}
            {payment.reference.trim() !== "" ? (
              <>
                <span>Referensi</span><span>{payment.reference}</span>
              </>
            ) : null}
          </div>
        </div>
      </section>

      {payment.allocations.length > 0 ? (
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

      <div className="kwitansi-amount">
        <div>
          <strong>Jumlah: {formatIDR(payment.amount)}</strong>
        </div>
        <div className="terbilang">Terbilang: {terbilangRupiah(payment.amount)}</div>
      </div>

      {payment.notes.trim() !== "" ? (
        <div className="kwitansi-note">
          <strong>Catatan:</strong> {payment.notes}
        </div>
      ) : null}
    </PrintLayout>
  );
}
