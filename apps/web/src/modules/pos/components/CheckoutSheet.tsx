import { useState } from "react";
import { Link } from "react-router-dom";
import { CheckCircle2, X } from "lucide-react";
import {
  Button,
  Checkbox,
  DateInput,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/shared/components/ui";
import { Numpad } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { formatQty } from "@/modules/pos/lib/qty";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { lineTotal } from "@/modules/pos/lib/cart";
import { NO_PAY_PERMISSION_MESSAGE } from "@/modules/pos/lib/checkout";
import { hideDiscountOption, invoiceHref, paymentProofHref, posReceiptHref } from "@/modules/pos/lib/receipt-links";
import { HIDE_DISCOUNT_UNPAID_NOTE } from "@/modules/print/lib/sales-document";
import { quickCashOptions } from "@/modules/pos/lib/quick-cash";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import { BankAccountSelect } from "@/modules/finance/components/BankAccountSelect";
import { payMethodLabel, type CheckoutMode, type PayMethod } from "@/modules/pos/types";

const CHOICE =
  "flex h-14 flex-1 touch-manipulation items-center justify-center rounded-lg border px-3 text-base font-semibold transition-[transform,background-color] duration-100 select-none active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50";

/** Pilihan tunggal besar (ToggleGroup sentuh): tombol aria-pressed, ≥ 56px. */
function Choice<T extends string>({
  label,
  options,
  value,
  disabled,
  onChange,
}: {
  label: string;
  options: { value: T; label: string; disabled?: boolean; describedBy?: string }[];
  value: T;
  disabled: boolean;
  onChange: (v: T) => void;
}) {
  return (
    <div role="group" aria-label={label} className="flex gap-2">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          disabled={disabled || o.disabled}
          aria-describedby={o.describedBy}
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            CHOICE,
            value === o.value ? "border-primary bg-accent text-accent-foreground ring-1 ring-primary" : "border-input bg-card text-foreground",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

const digitsOnly = (s: string) => s.replace(/\D/g, "");

function PaymentForm({ pos }: { pos: PosCartApi }) {
  const { mode, method, processing, estimateTotal, tendered } = pos;
  const addresses = pos.customerId !== null ? (pos.customerDetail?.addresses ?? []).filter((a) => a.status !== "archived") : [];
  // Setelah dibuka / memilih nominal cepat, ketukan angka pertama MENGGANTI nilai (bukan menambah).
  const [pristine, setPristine] = useState(true);

  return (
    <div className="space-y-4">
      {pos.resumeStep && (
        <Notice tone="info" title="Checkout sebelumnya belum selesai">
          Dilanjutkan dari langkah: {pos.resumeStep}. Dokumen yang sudah dibuat tidak digandakan.
        </Notice>
      )}
      <div className="rounded-xl bg-muted px-4 py-3">
        <p className="text-sm text-muted-foreground">Total (estimasi)</p>
        <p className="tabular font-display text-4xl font-bold text-foreground">{formatIDR(estimateTotal)}</p>
      </div>

      <Choice<CheckoutMode>
        label="Waktu pembayaran"
        value={mode}
        disabled={processing}
        onChange={pos.setMode}
        options={[
          { value: "pay_now", label: "Bayar Sekarang" },
          {
            value: "pay_later",
            label: "Bayar Nanti",
            // Termin net ditolak server untuk walk-in / pelanggan tanpa telepon.
            disabled: pos.payLaterBlock !== null && mode !== "pay_later",
            describedBy: pos.payLaterBlock !== null ? "pos-pay-later-hint" : undefined,
          },
        ]}
      />
      {pos.payLaterBlock !== null && (
        <p id="pos-pay-later-hint" className="-mt-2 text-xs text-muted-foreground">
          {pos.payLaterBlock}
        </p>
      )}

      {mode === "pay_now" && !pos.canPay && (
        <Notice tone="warning" title="Bayar sekarang tidak tersedia untuk akun ini">
          {NO_PAY_PERMISSION_MESSAGE}
        </Notice>
      )}

      {mode === "pay_later" ? (
        <>
          <div>
            <label htmlFor="pos-due-date" className="mb-1.5 block text-sm font-medium text-foreground">
              Jatuh Tempo <span aria-hidden className="text-destructive">*</span>
            </label>
            <DateInput
              id="pos-due-date"
              className="h-12 text-base"
              value={pos.dueDate}
              min={new Date().toISOString().split("T")[0]}
              onChange={(e) => pos.setDueDate(e.target.value)}
              disabled={processing}
            />
            <p className="mt-1 text-xs text-muted-foreground">Order dicatat sebagai piutang termin net.</p>
          </div>
        </>
      ) : (
        <>
          <Choice<PayMethod>
            label="Metode"
            value={method}
            disabled={processing}
            onChange={(m) => {
              pos.setMethod(m);
              if (m === "transfer") pos.setTendered(String(estimateTotal));
              setPristine(true);
            }}
            options={[
              { value: "cash", label: "Tunai" },
              { value: "transfer", label: "Transfer" },
            ]}
          />
          {method === "cash" ? (
            <div className="space-y-3">
              <div>
                <label htmlFor="pos-tendered" className="mb-1.5 block text-sm font-medium text-foreground">
                  Tunai Diterima <span aria-hidden className="text-destructive">*</span>
                </label>
                {/* inputMode="none": keyboard layar tidak muncul — nominal diisi lewat Numpad (keyboard fisik tetap bisa). */}
                <input
                  id="pos-tendered"
                  inputMode="none"
                  autoComplete="off"
                  value={tendered === "" ? "" : formatNumber(Number(tendered))}
                  onChange={(e) => {
                    setPristine(false);
                    pos.setTendered(digitsOnly(e.target.value));
                  }}
                  placeholder="Nominal tunai…"
                  disabled={processing}
                  className="tabular h-14 w-full rounded-lg border border-input bg-card px-4 text-right font-display text-3xl font-semibold text-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none disabled:bg-muted"
                />
              </div>
              <div className="flex flex-wrap gap-2">
                {quickCashOptions(estimateTotal).map((o) => (
                  <button
                    key={`${o.kind}-${o.amount}`}
                    type="button"
                    disabled={processing}
                    onClick={() => {
                      pos.setTendered(String(o.amount));
                      setPristine(true);
                    }}
                    className="flex h-12 flex-1 touch-manipulation items-center justify-center rounded-lg border border-input bg-card px-3 text-sm font-semibold whitespace-nowrap text-foreground select-none active:bg-accent disabled:opacity-50"
                  >
                    {o.kind === "exact" ? "Uang pas" : formatNumber(o.amount)}
                  </button>
                ))}
              </div>
              <Numpad
                value={pristine ? "" : tendered}
                onChange={(next) => {
                  setPristine(false);
                  pos.setTendered(next);
                }}
                onEnter={() => void pos.handleCheckout()}
                enterLabel="Proses"
                maxLength={11}
                disabled={processing}
                aria-label="Papan angka uang diterima"
              />
              <div className="flex items-baseline justify-between rounded-xl border border-border px-4 py-3">
                <span className="text-sm text-muted-foreground">Kembalian (estimasi)</span>
                <span
                  className={cn(
                    "tabular font-display text-3xl font-bold",
                    pos.changePreview < 0 ? "text-destructive" : "text-foreground",
                  )}
                >
                  {formatIDR(pos.changePreview)}
                </span>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              {pos.bankAccounts.required && (
                <div>
                  <label htmlFor="pos-bank-account" className="mb-1.5 block text-sm font-medium text-foreground">
                    Rekening tujuan <span aria-hidden className="text-destructive">*</span>
                  </label>
                  <BankAccountSelect
                    id="pos-bank-account"
                    options={pos.bankAccounts.active}
                    all={pos.bankAccounts.all}
                    value={pos.bankAccountId}
                    onChange={pos.setBankAccountId}
                    disabled={processing || pos.started}
                  />
                </div>
              )}
              <Notice tone="info" title="Transfer pas">
                Nominal transfer mengikuti total server ({formatIDR(estimateTotal)} estimasi) — tanpa kembalian.
              </Notice>
            </div>
          )}
        </>
      )}
      <div className="rounded-xl border border-border p-3" data-testid="pos-tax-controls">
        <label className="flex min-h-12 cursor-pointer items-center gap-3 text-sm font-medium text-foreground">
          <Checkbox
            id="pos-taxed"
            checked={pos.taxType !== "none"}
            disabled={processing || pos.started}
            onCheckedChange={(v) => pos.setTaxed(v === true)}
            className="size-6"
          />
          Kena pajak (PPN)
        </label>
        {pos.taxType !== "none" && (
          <div className="mt-2 space-y-2">
            <label className="flex min-h-12 cursor-pointer items-center gap-3 text-sm text-foreground">
              <Checkbox
                id="pos-tax-include"
                checked={pos.taxType === "include"}
                disabled={processing || pos.started}
                onCheckedChange={(v) => pos.setTaxIncluded(v === true)}
                className="size-6"
              />
              Harga sudah termasuk pajak
            </label>
            <div className="flex items-center gap-3">
              <label htmlFor="pos-tax-rate" className="text-sm text-muted-foreground">
                Tarif (%)
              </label>
              <input
                id="pos-tax-rate"
                inputMode="decimal"
                value={String(pos.taxRate)}
                disabled={processing || pos.started}
                onChange={(e) => {
                  const n = Number(e.target.value.replace(",", "."));
                  pos.setTaxRate(Number.isFinite(n) ? Math.min(100, Math.max(0, n)) : 0);
                }}
                className="h-12 w-24 rounded-lg border border-input bg-card px-3 text-right text-base text-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none disabled:bg-muted"
              />
            </div>
            <p data-testid="pos-tax-summary" className="text-sm text-muted-foreground">
              DPP {formatIDR(pos.totals.base)} Â· PPN {pos.taxRate}% {formatIDR(pos.totals.tax)} Â· Total {formatIDR(pos.totals.total)}
            </p>
          </div>
        )}
      </div>
      <div>
        <label htmlFor="pos-notes" className="mb-1.5 block text-sm font-medium text-foreground">
          Catatan transaksi
        </label>
        <textarea
          id="pos-notes"
          rows={2}
          maxLength={1000}
          value={pos.notes}
          onChange={(e) => pos.setNotes(e.target.value)}
          disabled={processing || pos.started}
          placeholder="Opsional — tersimpan di order"
          className="w-full rounded-lg border border-input bg-card px-3 py-2 text-base text-foreground placeholder:text-muted-foreground/70 focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none disabled:bg-muted"
        />
      </div>
      {addresses.length > 0 && (
        <div>
          <label htmlFor="pos-ship-to" className="mb-1.5 block text-sm font-medium text-foreground">
            Kirim ke alamat
          </label>
          <select
            id="pos-ship-to"
            value={pos.shipToAddressId ?? ""}
            onChange={(e) => pos.setShipToAddressId(e.target.value === "" ? null : Number(e.target.value))}
            disabled={processing || pos.started}
            className="h-12 w-full rounded-lg border border-input bg-card px-3 text-base text-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none disabled:bg-muted"
          >
            <option value="">Tanpa alamat kirim (bawa sendiri)</option>
            {addresses.map((a) => (
              <option key={a.id} value={a.id}>
                {(a.label || "Alamat") + " — " + a.text}
              </option>
            ))}
          </select>
        </div>
      )}
      {pos.checkoutError && <Notice tone="danger" title={pos.checkoutError} />}
    </div>
  );
}

/**
 * Opsi cetak "Sembunyikan diskon" transaksi selesai: tampil hanya bila ada
 * diskon (baris atau harga member), berlaku hanya bila lunas (ADR-0018 D12).
 */
function printHideDiscount(pos: PosCartApi) {
  const c = pos.completed;
  const option = hideDiscountOption({
    hasDiscount: pos.memberSavings > 0 || pos.cart.some((l) => l.discountPct > 0 || l.discountNominal > 0),
    payLater: c?.payLater ?? true,
    paymentPending: c?.paymentPending ?? true,
    change: c?.change ?? 0,
  });
  return { option, hide: pos.hideDiscount && option.allowed };
}

function CompletedView({ pos }: { pos: PosCartApi }) {
  const c = pos.completed;
  if (!c) return null;
  const { option, hide } = printHideDiscount(pos);
  const row = (label: string, value: string, strong = false) => (
    <div className="flex justify-between gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={cn("font-semibold text-foreground", strong && "tabular font-display text-lg")}>{value}</dd>
    </div>
  );
  return (
    <div className="space-y-4">
      <div className="flex flex-col items-center gap-2 py-2 text-center">
        <CheckCircle2 aria-hidden className="size-14 text-success" />
        <p className="font-display text-2xl font-semibold text-foreground">
          {c.payLater ? "Order terkonfirmasi" : "Pembayaran selesai"}
        </p>
      </div>
      <dl className="space-y-2 rounded-xl border border-border p-4 text-sm">
        {row("No. Order", c.orderNumber)}
        {row("Metode", c.payLater ? payMethodLabel("pay_later") : payMethodLabel(c.method))}
        {c.payLater && c.dueDate && (
          <>
            {row("Termin", "Net")}
            {row("Jatuh Tempo", c.dueDate)}
          </>
        )}
        {row("Total (server)", formatIDR(c.grandTotal), true)}
        {!c.payLater && (
          <>
            {row("Diterima", formatIDR(c.tendered))}
            <div className="flex items-baseline justify-between gap-3 border-t border-border pt-2">
              <dt className="text-muted-foreground">Kembalian</dt>
              <dd className="tabular font-display text-3xl font-bold text-foreground">{formatIDR(c.change)}</dd>
            </div>
          </>
        )}
      </dl>
      {c.payLater && (
        <Notice tone="info" title="Piutang tercatat">
          Tagih order {c.orderNumber} sebelum {c.dueDate} — tanpa pembayaran yang dicatat sekarang.
        </Notice>
      )}
      {c.change < 0 && !c.payLater && (
        <Notice tone="warning" title="Total server melebihi tunai">
          Selisih {formatIDR(-c.change)} harus ditagih manual ke pembeli.
        </Notice>
      )}
      {c.paymentPending && (
        <Notice tone="warning" title="Pembayaran belum tercatat">
          Catat pembayaran lunas order {c.orderNumber} manual di{" "}
          <Link to={ROUTE_PATHS.payments} className="font-semibold text-primary underline">
            menu Pembayaran
          </Link>
          .
        </Notice>
      )}
      <div className="rounded-xl border border-border p-4">
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm font-semibold text-foreground">Struk</p>
          {option.visible && (
            <label
              htmlFor="pos-hide-discount"
              className={cn(
                "flex min-h-12 items-center gap-2 text-sm text-muted-foreground",
                option.allowed ? "cursor-pointer" : "cursor-not-allowed opacity-60",
              )}
            >
              <Checkbox
                id="pos-hide-discount"
                checked={hide}
                disabled={!option.allowed}
                aria-describedby={option.allowed ? undefined : "pos-hide-discount-note"}
                onCheckedChange={(v) => pos.setHideDiscount(v === true)}
                className="size-6"
              />
              Sembunyikan diskon (harga penuh)
            </label>
          )}
        </div>
        {option.visible && !option.allowed && (
          <p id="pos-hide-discount-note" className="text-xs text-muted-foreground">
            Hanya untuk transaksi lunas — {HIDE_DISCOUNT_UNPAID_NOTE.toLowerCase()}.
          </p>
        )}
        <ul className="mt-1 divide-y divide-border text-sm">
          {pos.cart.map((l) => {
            const unit = hide ? l.unitPrice : pos.displayUnitPrice(l);
            return (
              <li key={l.key} className="flex justify-between gap-3 py-1.5">
                <span className="min-w-0 truncate text-muted-foreground">
                  {l.productName} × {formatQty(l.qty)}
                </span>
                <span className="tabular font-semibold text-foreground">
                  {formatIDR(
                    lineTotal({
                      qty: l.qty,
                      unitPrice: unit,
                      discountPct: hide ? 0 : l.discountPct,
                      discountNominal: hide ? 0 : l.discountNominal,
                    }),
                  )}
                </span>
              </li>
            );
          })}
        </ul>
        {!hide && pos.memberSavings > 0 && (
          <p className="mt-2 text-right text-xs font-semibold text-success">Termasuk hemat member {formatIDR(pos.memberSavings)}</p>
        )}
      </div>
    </div>
  );
}

/**
 * Sheet Bayar di sisi kanan selebar panel keranjang (total tetap terlihat). Menampilkan form
 * pembayaran, lalu layar selesai. Logika checkout ada di use-pos-cart / lib/checkout.
 */
export function CheckoutSheet({ pos }: { pos: PosCartApi }) {
  const { completed, processing, mode, started } = pos;
  const locked = processing || completed !== null;
  const can = useAuthStore((st) => st.can);
  const link = {
    orderId: completed?.orderId ?? 0,
    paymentId: completed?.paymentId ?? null,
    // Hanya pilihan yang berlaku (lunas + ada diskon) yang ikut ke tautan cetak.
    hideDiscount: printHideDiscount(pos).hide,
  };
  const proofHref = completed && can("payment.view") ? paymentProofHref(link) : null;
  const title = completed ? "Transaksi Berhasil" : mode === "pay_later" ? "Bayar Nanti" : "Pembayaran";
  const description = completed
    ? `Order ${completed.orderNumber} selesai diproses.`
    : mode === "pay_later"
      ? `Piutang termin net ${formatIDR(pos.estimateTotal)} (estimasi) — tanpa pembayaran sekarang.`
      : `Estimasi ${formatIDR(pos.estimateTotal)} — total final dihitung server.`;

  return (
    <Sheet open={pos.checkoutOpen} onOpenChange={(next) => !next && !locked && pos.setCheckoutOpen(false)}>
      <SheetContent
        side="right"
        showCloseButton={false}
        onInteractOutside={(e) => locked && e.preventDefault()}
        onEscapeKeyDown={(e) => locked && e.preventDefault()}
        className="flex w-full gap-0 bg-card p-0 sm:max-w-none lg:w-90 xl:w-100"
      >
        <SheetHeader className="flex-row items-start justify-between gap-3 border-b border-border p-4 text-left">
          <div className="min-w-0">
            <SheetTitle className="text-xl font-semibold">{title}</SheetTitle>
            <SheetDescription className="text-sm">{description}</SheetDescription>
          </div>
          {!completed && (
            <button
              type="button"
              aria-label="Tutup"
              disabled={processing}
              onClick={() => pos.setCheckoutOpen(false)}
              className="-mt-1 -mr-1 flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-muted-foreground select-none active:bg-accent disabled:opacity-50"
            >
              <X aria-hidden className="size-6" />
            </button>
          )}
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
          {completed ? <CompletedView pos={pos} /> : <PaymentForm pos={pos} />}
        </div>
        <SheetFooter className="flex-none flex-row gap-3 border-t border-border bg-card p-4">
          {completed ? (
            <>
              <Button asChild variant="secondary" className="h-14 flex-1 text-base">
                <a href={posReceiptHref(link)} target="_blank" rel="noreferrer">
                  Cetak Struk
                </a>
              </Button>
              <Button className="h-14 flex-1 text-base" onClick={pos.newTransaction}>
                Transaksi Baru
              </Button>
            </>
          ) : (
            <>
              <Button
                variant="secondary"
                className="h-14 px-5 text-base"
                disabled={processing}
                onClick={() => pos.setCheckoutOpen(false)}
              >
                Batal
              </Button>
              <Button
                className="h-14 flex-1 text-lg font-semibold"
                disabled={processing || (mode === "pay_now" && !pos.canPay)}
                onClick={() => void pos.handleCheckout()}
              >
                {processing ? "Memproses…" : started ? "Lanjutkan" : mode === "pay_later" ? "Buat Piutang" : "Proses Bayar"}
              </Button>
            </>
          )}
        </SheetFooter>
        {completed && (can("sales.view") || proofHref !== null) && (
          <div className="flex flex-none justify-center gap-6 border-t border-border bg-card px-4 pt-2">
            {can("sales.view") && (
              <a
                href={invoiceHref(link)}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-12 items-center text-sm font-medium text-primary underline underline-offset-4"
              >
                {completed.payLater ? "Cetak Tagihan" : "Cetak Nota"}
              </a>
            )}
            {proofHref !== null && (
              <a
                href={proofHref}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-12 items-center text-sm font-medium text-primary underline underline-offset-4"
              >
                Bukti Bayar
              </a>
            )}
          </div>
        )}
        {completed && (
          <div className="flex-none border-t border-border bg-card px-4 py-2 text-center">
            <Link
              to={`${ROUTE_PATHS.salesOrders}/${completed.orderId}`}
              className="inline-flex h-12 items-center text-sm font-medium text-primary underline underline-offset-4"
            >
              Lihat Order
            </Link>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
