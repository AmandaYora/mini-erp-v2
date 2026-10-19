import { useState } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle, ChevronRight, Info, ShoppingCart, Trash2 } from "lucide-react";
import { Button, ConfirmDialog, Popover, PopoverContent, PopoverTrigger } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { CartLineRow } from "@/modules/pos/components/CartLineRow";
import { CustomerSheet } from "@/modules/pos/components/CustomerSheet";
import { PosCustomerPanel } from "@/modules/pos/components/PosCustomerPanel";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";

/** Satu baris status izin di kepala keranjang; ketuk untuk rincian (Popover). */
function PermissionStatus({ pos }: { pos: PosCartApi }) {
  const { readiness, canPay } = pos;
  const blocked = !readiness.canCheckout;
  const count = readiness.notices.length + (canPay ? 0 : 1) + (blocked ? 1 : 0);
  if (count === 0) return null;
  const summary = blocked
    ? "Checkout tidak tersedia untuk akun ini"
    : `Fitur dibatasi izin akun (${count} catatan)`;
  const Icon = blocked ? AlertTriangle : Info;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn(
            "flex min-h-12 w-full touch-manipulation items-center gap-2 border-b px-4 text-left text-sm font-medium select-none",
            blocked
              ? "border-destructive/25 bg-destructive-soft text-destructive"
              : "border-warning/25 bg-warning-soft text-warning",
          )}
        >
          <Icon aria-hidden className="size-4 flex-none" />
          <span className="flex-1">{summary}</span>
          <ChevronRight aria-hidden className="size-4 flex-none" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 space-y-2 p-3 text-sm">
        {blocked && (
          <Notice tone="danger" title="Checkout tidak tersedia untuk akun ini">
            Izin yang dibutuhkan belum ada: {readiness.missingCore.map((m) => m.label).join(", ")}. Hubungi administrator untuk
            menambahkan izin tersebut.
          </Notice>
        )}
        {!canPay && (
          <Notice tone="warning" title="Tanpa izin pencatatan pembayaran">
            Order tetap bisa dibuat dan dikonfirmasi, tetapi pembayaran harus dicatat manual di{" "}
            <Link to={ROUTE_PATHS.payments} className="font-semibold text-primary underline">
              menu Pembayaran
            </Link>
            .
          </Notice>
        )}
        {readiness.notices.length > 0 && (
          <Notice tone="info" title="Beberapa fitur POS dibatasi izin akun">
            <ul className="list-disc pl-5">
              {readiness.notices.map((n) => (
                <li key={n.perm}>{n.text}</li>
              ))}
            </ul>
          </Notice>
        )}
      </PopoverContent>
    </Popover>
  );
}

/** Panel keranjang tetap di kanan: pelanggan, baris, ringkasan, TOTAL besar, BAYAR. */
export function CartPanel({ pos }: { pos: PosCartApi }) {
  const [customerOpen, setCustomerOpen] = useState(false);
  const [confirmClear, setConfirmClear] = useState(false);
  const { cart, totals, started, completed, readiness } = pos;

  return (
    <aside
      aria-label="Keranjang"
      className="flex min-h-0 w-90 flex-none flex-col border-l border-border bg-card xl:w-100"
    >
      <PosCustomerPanel
        customer={pos.customerDetail}
        memberName={pos.memberName}
        quote={pos.quote}
        quoteStatus={pos.quoteStatus}
        quoteError={pos.quoteError}
        savings={pos.memberSavings}
        locked={started}
        onChange={() => setCustomerOpen(true)}
      />
      <PermissionStatus pos={pos} />

      {started && completed === null && (
        <div className="border-b border-border p-3">
          <Notice tone="info" title="Checkout sedang berjalan">
            Isi keranjang terkunci agar sama dengan order di server. Lanjutkan pembayaran, atau batalkan transaksi ini.
            <div className="mt-2 flex gap-2">
              <Button className="h-12 flex-1 text-base" onClick={pos.openCheckout}>
                Lanjutkan
              </Button>
              <Button variant="secondary" className="h-12 flex-1 text-base" onClick={() => void pos.abandonTransaction()}>
                Batalkan Transaksi
              </Button>
            </div>
          </Notice>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
        {cart.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 px-6 text-center text-muted-foreground">
            <ShoppingCart aria-hidden className="size-10" />
            <p className="text-base font-semibold text-foreground">Keranjang kosong</p>
            <p className="text-sm">Ketuk produk di kiri atau pindai barcode untuk menambah.</p>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {cart.map((l) => (
              <CartLineRow key={l.key} line={l} pos={pos} />
            ))}
          </ul>
        )}
      </div>

      <div className="flex-none space-y-1.5 border-t border-border bg-card p-4">
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>Subtotal ({formatNumber(totals.itemCount)} item)</span>
          <span className="tabular">{formatIDR(totals.subtotal)}</span>
        </div>
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>Diskon</span>
          <span className="tabular">{totals.discount > 0 ? `−${formatIDR(totals.discount)}` : formatIDR(0)}</span>
        </div>
        {pos.taxType !== "none" && (
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <span>
              PPN {pos.taxRate}% {pos.taxType === "include" ? "(termasuk)" : ""}
            </span>
            <span className="tabular">{formatIDR(totals.tax)}</span>
          </div>
        )}
        <div className="flex items-baseline justify-between gap-2 pt-1">
          <span className="text-sm font-semibold text-muted-foreground">TOTAL</span>
          <span className="tabular font-display text-4xl font-bold text-foreground">{formatIDR(totals.total)}</span>
        </div>
        <p className="text-right text-2xs text-muted-foreground">Estimasi — total final dihitung server.</p>
        <div className="flex gap-3 pt-2">
          <Button
            variant="secondary"
            className="h-14 flex-none px-4 text-base"
            disabled={cart.length === 0 || started}
            onClick={() => setConfirmClear(true)}
            aria-label="Kosongkan keranjang"
          >
            <Trash2 aria-hidden className="size-5" />
            Kosongkan
          </Button>
          <Button
            className="h-14 flex-1 text-lg font-semibold"
            disabled={cart.length === 0 || !readiness.canCheckout}
            onClick={pos.openCheckout}
          >
            {started ? "Lanjutkan Checkout" : "Bayar"} · {formatIDR(totals.total)}
            <ChevronRight aria-hidden className="size-5" />
          </Button>
        </div>
      </div>

      <CustomerSheet
        open={customerOpen}
        selectedId={pos.customerId}
        requirePhone={pos.mode === "pay_later"}
        onSelect={pos.setCustomerId}
        onClose={() => setCustomerOpen(false)}
      />
      <ConfirmDialog
        open={confirmClear}
        title="Kosongkan keranjang?"
        message="Semua barang di keranjang akan dihapus."
        confirmLabel="Kosongkan"
        onConfirm={() => {
          pos.clearCart();
          setConfirmClear(false);
        }}
        onCancel={() => setConfirmClear(false)}
      />
    </aside>
  );
}
