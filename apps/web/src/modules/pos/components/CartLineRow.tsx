import { useState, type KeyboardEvent } from "react";
import { ChevronRight, Minus, Plus, X } from "lucide-react";
import { Button, Popover, PopoverContent, PopoverTrigger } from "@/shared/components/ui";
import { Numpad } from "@/shared/components/patterns";
import { cn } from "@/shared/lib/cn";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { lineDiscount } from "@/modules/pos/lib/cart";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import { acceptQtyDraft, canDecrease, formatQty, parseQtyInput } from "@/modules/pos/lib/qty";
import type { CartLine } from "@/modules/pos/types";

const STEP =
  "flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg border border-input bg-card text-foreground transition-[transform,background-color] duration-100 select-none active:scale-[0.98] active:bg-accent disabled:pointer-events-none disabled:opacity-40";

/**
 * Qty: ketuk angka membuka Numpad (tanpa keyboard sistem). Desimal boleh (temuan 7, mis. 0,8 m kabel):
 * tombol koma di Numpad, atau keyboard fisik — angka, "," atau ".", Backspace, Enter.
 */
function QtyPopover({ line, disabled, onCommit }: { line: CartLine; disabled: boolean; onCommit: (qty: number) => void }) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const parsed = draft === "" ? null : parseQtyInput(draft);
  function edit(next: string) {
    if (acceptQtyDraft(next)) setDraft(next);
  }
  function commit() {
    if (parsed !== null) onCommit(parsed);
    setOpen(false);
  }
  function onKeyDown(e: KeyboardEvent) {
    if (/^\d$/.test(e.key)) edit(draft === "0" ? e.key : draft + e.key);
    else if ((e.key === "," || e.key === ".") && !draft.includes(".")) edit(draft === "" ? "0." : `${draft}.`);
    else if (e.key === "Backspace") setDraft(draft.slice(0, -1));
    else if (e.key === "Enter") commit();
    else return;
    e.preventDefault();
  }
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) setDraft("");
        setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          aria-label={`Ubah qty ${line.productName}`}
          className="tabular flex h-12 min-w-14 touch-manipulation items-center justify-center rounded-lg px-2 font-display text-xl font-semibold text-foreground select-none active:bg-accent disabled:opacity-60"
        >
          {formatQty(line.qty)}
        </button>
      </PopoverTrigger>
      <PopoverContent align="center" className="w-80 space-y-3 p-3" onKeyDown={onKeyDown}>
        <div className="flex items-baseline justify-between rounded-lg bg-muted px-4 py-2">
          <span className="text-sm text-muted-foreground">Qty ({line.uom})</span>
          <span className={cn("tabular font-display text-3xl font-semibold", draft === "" && "text-muted-foreground")}>
            {draft === "" ? formatQty(line.qty) : draft.replace(".", ",")}
          </span>
        </div>
        <Numpad
          value={draft}
          onChange={edit}
          onEnter={commit}
          enterLabel="OK"
          allowDecimal
          decimalSeparator=","
          maxLength={10}
          aria-label="Papan angka qty"
        />
      </PopoverContent>
    </Popover>
  );
}

/** Diskon baris: persen dan/atau nominal lewat Numpad; batas harga minimum dijelaskan. */
function DiscountPopover({ line, pos }: { line: CartLine; pos: PosCartApi }) {
  const [open, setOpen] = useState(false);
  const [pct, setPct] = useState("");
  const [nominal, setNominal] = useState("");
  const [field, setField] = useState<"pct" | "nominal">("pct");
  const applied = lineDiscount({
    qty: line.qty,
    unitPrice: pos.displayUnitPrice(line),
    discountPct: line.discountPct,
    discountNominal: line.discountNominal,
  });
  const limited = line.minSellingPrice > 0 && pos.memberPriceOf(line.productId) === null;

  function apply() {
    pos.setLineDiscount(line.key, { discountPct: Number(pct || 0), discountNominal: Number(nominal || 0) });
    setOpen(false);
  }
  function reset() {
    pos.setLineDiscount(line.key, { discountPct: 0, discountNominal: 0 });
    setOpen(false);
  }

  const fieldBtn = (id: "pct" | "nominal", label: string, text: string) => (
    <button
      type="button"
      aria-pressed={field === id}
      onClick={() => setField(id)}
      className={cn(
        "flex min-h-14 flex-1 touch-manipulation flex-col items-start justify-center rounded-lg border px-3 text-left select-none",
        field === id ? "border-primary bg-accent ring-1 ring-primary" : "border-input bg-card",
      )}
    >
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="tabular font-display text-xl font-semibold text-foreground">{text}</span>
    </button>
  );

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) {
          setPct(line.discountPct > 0 ? String(line.discountPct) : "");
          setNominal(line.discountNominal > 0 ? String(line.discountNominal) : "");
          setField("pct");
        }
        setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={pos.started}
          aria-label={`Diskon ${line.productName}`}
          className="flex h-12 touch-manipulation items-center gap-1 rounded-lg px-2 text-sm font-medium text-primary select-none active:bg-accent disabled:opacity-50"
        >
          {applied > 0 ? `Diskon −${formatIDR(applied)}` : "Diskon"}
          <ChevronRight aria-hidden className="size-4" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 space-y-3 p-3">
        <p className="text-sm font-semibold text-foreground">Diskon {line.productName}</p>
        <div className="flex gap-2">
          {fieldBtn("pct", "Persen (%)", pct === "" ? "0" : pct.replace(".", ","))}
          {fieldBtn("nominal", "Nominal (Rp, per baris)", nominal === "" ? "0" : formatNumber(Number(nominal)))}
        </div>
        <Numpad
          value={field === "pct" ? pct : nominal}
          onChange={field === "pct" ? setPct : setNominal}
          onEnter={apply}
          enterLabel="Terapkan"
          allowDecimal={field === "pct"}
          maxLength={field === "pct" ? 6 : 10}
          aria-label="Papan angka diskon"
        />
        {limited && (
          <p className="text-xs text-muted-foreground">
            Potongan maksimum {formatIDR(pos.discountCeilingOf(line))} agar harga tidak di bawah harga jual minimum.
          </p>
        )}
        {(line.discountPct > 0 || line.discountNominal > 0) && (
          <Button variant="ghost" className="h-12 w-full text-base" onClick={reset}>
            Hapus diskon
          </Button>
        )}
      </PopoverContent>
    </Popover>
  );
}

export function CartLineRow({ line, pos }: { line: CartLine; pos: PosCartApi }) {
  const st = pos.holds.lineState[`${line.productId}:${line.variantId}`];
  const memberPrice = pos.memberPriceOf(line.productId);
  const quoteErr = pos.quoteLineErrors.get(line.productId);
  return (
    <li className="px-4 py-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="line-clamp-2 text-base leading-snug font-medium text-foreground">{line.productName}</p>
          <p className="text-xs text-muted-foreground">
            {line.variantCode} — {line.variantName} · {line.uom}
          </p>
        </div>
        <button
          type="button"
          disabled={pos.started}
          onClick={() => pos.removeLine(line.key)}
          aria-label={`Hapus ${line.productName}`}
          className="-mt-1 -mr-2 flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-muted-foreground select-none active:bg-destructive-soft active:text-destructive disabled:opacity-40"
        >
          <X aria-hidden className="size-5" />
        </button>
      </div>
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className={cn("tabular text-sm text-foreground", memberPrice !== null && "text-muted-foreground line-through")}>
            {formatIDR(line.unitPrice)}
          </p>
          {memberPrice !== null && <p className="tabular text-sm font-semibold text-success">Member {formatIDR(memberPrice)}</p>}
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            className={STEP}
            // Langkah 1; di bawah 1 (mis. 0,8) tombol nonaktif — ubah lewat angka, atau hapus baris.
            disabled={pos.started || !canDecrease(line.qty)}
            onClick={() => pos.changeQty(line.key, -1)}
            aria-label="Kurangi qty"
          >
            <Minus aria-hidden className="size-5" />
          </button>
          <QtyPopover line={line} disabled={pos.started} onCommit={(q) => pos.setQty(line.key, q)} />
          <button
            type="button"
            className={STEP}
            disabled={pos.started}
            onClick={() => pos.changeQty(line.key, 1)}
            aria-label="Tambah qty"
          >
            <Plus aria-hidden className="size-5" />
          </button>
        </div>
      </div>
      <div className="flex items-center justify-between gap-2">
        {pos.canDiscount ? <DiscountPopover line={line} pos={pos} /> : <span />}
        <p className="tabular font-display text-lg font-semibold text-foreground">{formatIDR(pos.estimateOf(line))}</p>
      </div>
      {quoteErr && <p className="text-xs font-semibold text-destructive">Harga member tidak dapat dihitung: {quoteErr}</p>}
      {st && st.shortfall > 0 && (
        <p className="text-xs font-semibold text-destructive">Stok kurang {formatQty(st.shortfall)} — kurangi qty.</p>
      )}
      {st?.error && <p className="text-xs font-semibold text-destructive">{st.error}</p>}
    </li>
  );
}
