// Struk termal POS 80mm (opsi 58mm).
//
// Dokumen yang dipegang pembeli: isi SAMA dengan pos-thermal-receipt-print-page.tsx
// sistem lama (identitas perusahaan); angka dari lib/pos-receipt + lib/sales-document.
// Data: sales order POS + riwayat pembayaran tersimpan order itu (cetak ulang
// konsisten). ?paymentId= hanya cadangan bila riwayat tak terbaca.
// Cetak: jembatan native Android bila tersedia, fallback window.print() (D6).

import { useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PRINT_LOGO_SRC } from "@/modules/print/components/print-assets";
import {
  formatPhoneLine,
  formatReceiptDateTime,
  formatReceiptQuantity,
  printItemName,
} from "@/modules/print/document";
import { receiptPayments, receiptSummary } from "@/modules/print/lib/pos-receipt";
import {
  effectiveHideDiscount,
  HIDE_DISCOUNT_UNPAID_NOTE,
  hideDiscountAllowed,
  lineDiscountTotal,
  linePrintSubtotal,
  linePrintUnitPrice,
  orderPrintDiscount,
  readHideDiscount,
  withHideDiscountParam,
} from "@/modules/print/lib/sales-document";
import { PrintProfileNotice } from "@/modules/print/components/PrintProfileNotice";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import { fetchPosReceipt, type PosReceiptData } from "@/modules/print/services/print-fetch";
import {
  isNativePrinterAvailable,
  printReceipt,
  type ThermalPrintPayload,
} from "@/modules/print/lib/native-printer";
import {
  LEGACY_UA_CSS,
  readThermalWidth,
  thermalStyles,
  type ThermalWidth,
} from "@/modules/print/styles";
import { useAsyncData } from "@/shared/hooks/use-async-data";

export default function PosThermalReceiptPage() {
  const { orderId } = useParams<{ orderId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, warning, loading: profileLoading, bankAccounts } = usePrintProfile();

  const [printing, setPrinting] = useState(false);

  const id = Number(orderId);
  const paymentIdParam = searchParams.get("paymentId");
  const width: ThermalWidth = readThermalWidth(searchParams);
  // SL-05: pilihan checkout ikut di tautan (?hideDiscount=1); toolbar bisa mengubahnya.
  // Berlaku hanya bila transaksi lunas (ADR-0018 D12) — dicek setelah order termuat.
  const hideRequested = readHideDiscount(searchParams);

  const { data, loading, error } = useAsyncData(
    async (): Promise<PosReceiptData> => {
      if (!Number.isInteger(id) || id <= 0) {
        throw new Error("ID order tidak valid.");
      }
      try {
        return await fetchPosReceipt(id, Number(paymentIdParam));
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat struk", msg);
        throw err;
      }
    },
    [id, paymentIdParam ?? ""],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat struk POS...</div>;
  }
  if (error !== null || data === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Order tidak ditemukan."}</div>;
  }
  const o = data.order;
  if (o.channel !== "pos") {
    return <div style={{ padding: "2rem" }}>Struk termal hanya untuk transaksi POS.</div>;
  }

  const hideAllowed = hideDiscountAllowed(o);
  const hideDiscount = effectiveHideDiscount(hideRequested, o);
  const payments = receiptPayments(data.payments);
  const s = receiptSummary(o, payments, hideDiscount);
  const printDiscount = orderPrintDiscount(o);
  const customerName = o.partyId > 0 && o.partyName.trim() !== "" ? o.partyName : "Umum";
  // Kasir = pencatat pembayaran (server: cashierName); pengguna yang mencetak hanya cadangan.
  const cashierName = o.cashierName?.trim() || user?.fullName?.trim() || "-";
  // Transfer (atau piutang) ke rekening tertentu (ADR-0026): cetak rekening itu saja;
  // selain itu daftar rekening dokumen seperti sistem lama.
  const chosenAccountId =
    payments.find((p) => (p.cashAccountId ?? 0) > 0)?.cashAccountId ??
    (s.statusLabel === "PIUTANG" ? (o.receiptAccountId ?? null) : null);
  const chosen = chosenAccountId ? (bankAccounts ?? []).find((b) => b.accountId === chosenAccountId) : undefined;
  const rekeningList = chosen
    ? [{ bank: chosen.bankName, holder: chosen.accountHolder, number: chosen.accountNumber }]
    : identity.bankAccounts;
  const rekeningHolders = Array.from(new Set(rekeningList.map((a) => a.holder.trim()).filter(Boolean)));
  const rekeningAccounts = rekeningList.map((a) => `(${a.bank} ${a.number})`).join(" / ");
  const rekeningLine =
    rekeningList.length > 0
      ? `REKENING : ${[rekeningHolders.join(" / "), rekeningAccounts].filter(Boolean).join(" ")}`
      : "";
  const returnNote = identity.returnNote !== "" ? identity.returnNote : "Barang yang sudah dibeli mengikuti kebijakan toko.";
  const thanksNote = identity.thanksNote !== "" ? identity.thanksNote : "Terima kasih";

  // Payload native: ANGKA FINAL yang sama dengan yang dirender di layar.
  function buildPayload(): ThermalPrintPayload {
    return {
      store: {
        name: identity.name,
        address: identity.tagline !== "" ? identity.tagline : undefined,
        phone: identity.phones.length > 0 ? formatPhoneLine(identity.phones[0]) : undefined,
        footer: thanksNote,
      },
      meta: {
        orderNumber: o.number,
        paymentNumber: s.paymentNumber !== "-" ? s.paymentNumber : undefined,
        datetime: formatReceiptDateTime(s.receiptDate),
        cashier: cashierName,
        customer: customerName,
        status: s.statusLabel,
      },
      items: o.items.map((line) => {
        const discount = lineDiscountTotal(line);
        return {
          name: printItemName(line),
          qty: line.qty,
          uom: line.uom,
          unitPrice: linePrintUnitPrice(line),
          lineTotal: linePrintSubtotal(line),
          discount: !hideDiscount && discount > 0 ? discount : undefined,
          note: line.note.trim() !== "" ? line.note : undefined,
        };
      }),
      summary: {
        subtotal: s.subtotal,
        discount: s.discount ?? undefined,
        tax: s.tax ?? undefined,
        total: s.total,
        paid: s.paid,
        tendered: s.tendered ?? undefined,
        change: s.change ?? undefined,
        outstanding: s.outstanding ?? undefined,
        method: s.methods,
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
        toast.danger("Gagal mencetak struk", result.message ?? "Coba lagi atau cek printer.");
      }
    } finally {
      setPrinting(false);
    }
  }

  return (
    <>
      <style>{LEGACY_UA_CSS + thermalStyles(width)}</style>
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
        {printDiscount > 0 ? (
          <label className="thermal-toolbar-option" title={hideAllowed ? undefined : HIDE_DISCOUNT_UNPAID_NOTE}>
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
                — hanya untuk struk lunas. {HIDE_DISCOUNT_UNPAID_NOTE}.
              </span>
            )}
          </label>
        ) : null}
        <button type="button" onClick={() => void handlePrint()} disabled={printing}>
          {printing ? "Mencetak…" : "Cetak Struk"}
        </button>
        <button type="button" onClick={() => window.close()}>
          Tutup
        </button>
      </div>
      <PrintProfileNotice corrupt={false} warning={warning} />
      <main className="thermal-receipt">
        <header className="thermal-center">
          <img alt="Logo" className="thermal-logo" src={PRINT_LOGO_SRC} />
          <div className="thermal-store">{identity.name}</div>
          {identity.tagline !== "" ? <div className="thermal-tagline">{identity.tagline}</div> : null}
          {identity.phones.map((phone, index) => (
            <div className="thermal-phone" key={`ph-${index}`}>
              HP : {formatPhoneLine(phone)}
            </div>
          ))}
          <div className="thermal-status">{s.statusLabel}</div>
        </header>

        <div className="thermal-separator" />

        <section>
          <div className="thermal-row"><span>No Order</span><span>{o.number}</span></div>
          <div className="thermal-row"><span>No Bayar</span><span>{s.paymentNumber}</span></div>
          <div className="thermal-row"><span>Tanggal</span><span>{formatReceiptDateTime(s.receiptDate)}</span></div>
          <div className="thermal-row"><span>Kasir</span><span>{cashierName}</span></div>
          <div className="thermal-row"><span>Pelanggan</span><span>{customerName}</span></div>
        </section>

        <div className="thermal-separator" />

        <section>
          {o.items.map((line) => {
            const discount = lineDiscountTotal(line);
            return (
              <div className="thermal-item" key={line.id}>
                <div className="thermal-item-name">{printItemName(line)}</div>
                <div className="thermal-item-meta">
                  <span>
                    {formatReceiptQuantity(line.qty)} {line.uom} x {formatIDR(linePrintUnitPrice(line))}
                  </span>
                  <span>{formatIDR(linePrintSubtotal(line))}</span>
                </div>
                {!hideDiscount && discount > 0 ? (
                  <div className="thermal-item-meta">
                    <span>Diskon</span>
                    <span>-{formatIDR(discount)}</span>
                  </div>
                ) : null}
              </div>
            );
          })}
        </section>

        <div className="thermal-separator" />

        <section>
          <div className="thermal-row"><span>Subtotal</span><span>{formatIDR(s.subtotal)}</span></div>
          {s.discount !== null ? (
            <div className="thermal-row"><span>Diskon</span><span>{formatIDR(s.discount)}</span></div>
          ) : null}
          {/* PPN di POS (K-03) tidak ada di sistem lama: struk ber-PPN merinci DPP dan tarifnya. DPP = TOTAL tercetak − PPN, jadi tetap menjumlah saat diskon disembunyikan. */}
          {s.tax !== null ? (
            <>
              <div className="thermal-row"><span>DPP</span><span>{formatIDR(s.total - s.tax)}</span></div>
              <div className="thermal-row"><span>{o.taxRate > 0 ? `PPN ${o.taxRate}%` : "PPN"}</span><span>{formatIDR(s.tax)}</span></div>
            </>
          ) : null}
          <div className="thermal-row thermal-total"><span>TOTAL</span><span>{formatIDR(s.total)}</span></div>
          <div className="thermal-row"><span>Dibayar</span><span>{formatIDR(s.paid)}</span></div>
          {s.tendered !== null ? (
            <>
              <div className="thermal-row"><span>Tunai</span><span>{formatIDR(s.tendered)}</span></div>
              <div className="thermal-row"><span>Kembalian</span><span>{formatIDR(s.change ?? 0)}</span></div>
            </>
          ) : null}
          {s.outstanding !== null ? (
            <div className="thermal-row"><span>Sisa</span><span>{formatIDR(s.outstanding)}</span></div>
          ) : null}
          <div className="thermal-row"><span>Metode</span><span>{s.methods}</span></div>
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
