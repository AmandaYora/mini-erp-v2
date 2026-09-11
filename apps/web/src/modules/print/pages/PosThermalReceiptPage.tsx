// Struk termal POS 58/80mm (Tahap E5).
//
// Data: sales order channel POS + satu pembayaran opsional (?paymentId=).
// Cetak: jembatan native Android bila tersedia, fallback window.print() (D6).
// Uang tunai diterima/kembalian tidak ada di view revamp — tidak ditampilkan
// (tidak ditebak).

import { useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { formatIDR, statusLabel } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PRINT_LOGO_SRC } from "@/modules/print/components/print-assets";
import { formatPhoneLine } from "@/modules/print/document";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import {
  fetchPaymentForPrint,
  fetchSalesOrderForPrint,
  type Payment,
  type SalesOrder,
} from "@/modules/print/services/print-fetch";
import {
  isNativePrinterAvailable,
  printReceipt,
  type ThermalPrintPayload,
} from "@/modules/print/lib/native-printer";
import {
  readThermalWidth,
  thermalStyles,
  type ThermalWidth,
} from "@/modules/print/styles";
import { useAsyncData } from "@/shared/hooks/use-async-data";

function formatQty(qty: number): string {
  return new Intl.NumberFormat("id-ID", { maximumFractionDigits: 3 }).format(qty);
}

function formatShortDateTime(v: string | null | undefined): string {
  if (!v) return "-";
  const d = new Date(v.length <= 10 ? `${v}T00:00:00` : v);
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", {
    day: "2-digit",
    month: "2-digit",
    year: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export default function PosThermalReceiptPage() {
  const { orderId } = useParams<{ orderId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, loading: profileLoading } = usePrintProfile();

  const [printing, setPrinting] = useState(false);

  const id = Number(orderId);
  const paymentIdParam = searchParams.get("paymentId");
  const paymentId = paymentIdParam !== null ? Number(paymentIdParam) : NaN;
  const width: ThermalWidth = readThermalWidth(searchParams);

  const { data, loading, error } = useAsyncData(
    async () => {
      if (!Number.isInteger(id) || id <= 0) {
        throw new Error("ID order tidak valid.");
      }
      try {
        const o = await fetchSalesOrderForPrint(id);
        let p: Payment | null = null;
        if (Number.isInteger(paymentId) && paymentId > 0) {
          p = await fetchPaymentForPrint(paymentId).catch(() => null);
        }
        return { order: o, payment: p };
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat struk", msg);
        throw err;
      }
    },
    [id, paymentIdParam ?? ""],
  );
  const order: SalesOrder | null = data?.order ?? null;
  const payment: Payment | null = data?.payment ?? null;

  const receiptDate = useMemo(() => {
    if (payment !== null) return payment.paidAt;
    if (order !== null) return order.orderDate;
    return null;
  }, [payment, order]);

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat struk POS…</div>;
  }
  if (error !== null || order === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Order tidak ditemukan."}</div>;
  }
  if (order.channel !== "pos") {
    return <div style={{ padding: "2rem" }}>Struk termal hanya untuk transaksi POS.</div>;
  }
  // Setelah guard di atas, order non-null — alias agar narrowing bertahan
  // di dalam closure buildPayload/handlePrint.
  const o: SalesOrder = order;

  const customerName = o.partyName.trim() !== "" ? o.partyName : "Umum";
  const cashierName = user?.fullName?.trim() !== "" ? (user?.fullName ?? "-") : "-";
  const statusStamp = payment !== null ? "LUNAS" : statusLabel(o.status).toUpperCase();
  const paidAmount = payment !== null ? payment.amount : 0;
  const outstanding = Math.max(0, o.grandTotal - paidAmount);
  const methodLine =
    payment !== null
      ? payment.method
      : o.paymentTerms === "net"
        ? "Tempo"
        : "-";
  const rekeningLine =
    identity.bankAccounts.length > 0
      ? `REKENING : ${identity.bankAccounts.map((a) => `(${a.bank} ${a.number})`).join(" / ")}`
      : "";
  const returnNote = identity.returnNote !== "" ? identity.returnNote : "Barang yang sudah dibeli mengikuti kebijakan toko.";
  const thanksNote = identity.thanksNote !== "" ? identity.thanksNote : "Terima kasih";

  function buildPayload(): ThermalPrintPayload {
    return {
      store: {
        name: identity.name,
        address: identity.tagline !== "" ? identity.tagline : undefined,
        phone:
          identity.phones.length > 0 ? formatPhoneLine(identity.phones[0]) : undefined,
        footer: thanksNote,
      },
      meta: {
        orderNumber: o.number,
        paymentNumber: payment !== null ? payment.number : undefined,
        datetime: formatShortDateTime(receiptDate),
        cashier: cashierName,
        customer: customerName,
        status: statusStamp,
      },
      items: o.items.map((line) => ({
        name: line.productName,
        qty: line.qty,
        uom: line.uom,
        unitPrice: line.unitPrice,
        lineTotal: line.lineTotal,
        discount:
          line.discountNominal > 0
            ? line.discountNominal
            : undefined,
        note: undefined,
      })),
      summary: {
        subtotal: o.subtotal,
        discount: o.discountTotal > 0 ? o.discountTotal : undefined,
        tax: o.taxTotal > 0 ? o.taxTotal : undefined,
        total: o.grandTotal,
        paid: payment !== null ? paidAmount : undefined,
        outstanding: payment !== null && outstanding > 0 ? outstanding : undefined,
        method: methodLine,
      },
    };
  }

  async function handlePrint() {
    if (!isNativePrinterAvailable()) {
      window.print();
      return;
    }
    setPrinting(true);
    try {
      const result = await printReceipt(buildPayload());
      if (!result.ok) {
        toast.danger(
          "Gagal mencetak struk",
          result.message ?? "Coba lagi atau cek printer.",
        );
      }
    } finally {
      setPrinting(false);
    }
  }

  return (
    <>
      <style>{thermalStyles(width)}</style>
      <div className="no-print thermal-toolbar">
        <label className="thermal-toolbar-option">
          <input
            type="radio"
            name="thermal-width"
            checked={width === 80}
            onChange={() => {
              const next = new URLSearchParams(searchParams);
              next.delete("w");
              setSearchParams(next, { replace: true });
            }}
          />
          <span>80mm</span>
        </label>
        <label className="thermal-toolbar-option">
          <input
            type="radio"
            name="thermal-width"
            checked={width === 58}
            onChange={() => {
              const next = new URLSearchParams(searchParams);
              next.set("w", "58");
              setSearchParams(next, { replace: true });
            }}
          />
          <span>58mm</span>
        </label>
        <button type="button" onClick={() => void handlePrint()} disabled={printing}>
          {printing ? "Mencetak…" : "Cetak Struk"}
        </button>
      </div>
      <main className="thermal-receipt">
        <header className="thermal-center">
          <img alt="Logo" className="thermal-logo" src={PRINT_LOGO_SRC} />
          <div className="thermal-store">{identity.name}</div>
          {identity.tagline !== "" ? (
            <div className="thermal-tagline">{identity.tagline}</div>
          ) : null}
          {identity.phones.map((phone, index) => (
            <div className="thermal-phone" key={`ph-${index}`}>
              HP : {formatPhoneLine(phone)}
            </div>
          ))}
          <div className="thermal-status">{statusStamp}</div>
        </header>

        <div className="thermal-separator" />

        <section>
          <div className="thermal-row"><span>No Order</span><span>{o.number}</span></div>
          {payment !== null ? (
            <div className="thermal-row"><span>No Bayar</span><span>{payment.number}</span></div>
          ) : null}
          <div className="thermal-row"><span>Tanggal</span><span>{formatShortDateTime(receiptDate)}</span></div>
          <div className="thermal-row"><span>Kasir</span><span>{cashierName}</span></div>
          <div className="thermal-row"><span>Pelanggan</span><span>{customerName}</span></div>
        </section>

        <div className="thermal-separator" />

        <section>
          {o.items.map((line) => (
            <div className="thermal-item" key={line.id}>
              <div className="thermal-item-name">{line.productName}</div>
              <div className="thermal-item-meta">
                <span>
                  {formatQty(line.qty)} {line.uom} x {formatIDR(line.unitPrice)}
                </span>
                <span>{formatIDR(line.lineTotal)}</span>
              </div>
              {line.discountNominal > 0 ? (
                <div className="thermal-item-meta">
                  <span>Diskon</span>
                  <span>-{formatIDR(line.discountNominal)}</span>
                </div>
              ) : null}
            </div>
          ))}
        </section>

        <div className="thermal-separator" />

        <section>
          <div className="thermal-row"><span>Subtotal</span><span>{formatIDR(o.subtotal)}</span></div>
          {o.discountTotal > 0 ? (
            <div className="thermal-row"><span>Diskon</span><span>{formatIDR(o.discountTotal)}</span></div>
          ) : null}
          {o.taxTotal > 0 ? (
            <div className="thermal-row"><span>Pajak</span><span>{formatIDR(o.taxTotal)}</span></div>
          ) : null}
          <div className="thermal-row thermal-total"><span>TOTAL</span><span>{formatIDR(o.grandTotal)}</span></div>
          {payment !== null ? (
            <div className="thermal-row"><span>Dibayar</span><span>{formatIDR(paidAmount)}</span></div>
          ) : null}
          {payment !== null && outstanding > 0 ? (
            <div className="thermal-row"><span>Sisa</span><span>{formatIDR(outstanding)}</span></div>
          ) : null}
          <div className="thermal-row"><span>Metode</span><span>{methodLine}</span></div>
        </section>

        <div className="thermal-separator" />

        <footer className="thermal-center">
          {rekeningLine !== "" ? <div className="thermal-rekening">{rekeningLine}</div> : null}
          <div className="thermal-note">{returnNote}</div>
          <div className="thermal-footer">{thanksNote}</div>
        </footer>
      </main>
    </>
  );
}
