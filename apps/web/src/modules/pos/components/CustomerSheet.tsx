import { useEffect, useState } from "react";
import { Check, Search, UserPlus, UserRound, X } from "lucide-react";
import {
  Button,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  Skeleton,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { newCustomerBody, validateNewCustomer } from "@/modules/pos/lib/new-customer";
import { posService } from "@/modules/pos/services/pos.service";
import { readApiError } from "@/modules/pos/lib/api-error";
import type { PosCustomer, PosMemberType } from "@/modules/pos/types";
import { formatApiError } from "@/shared/services/http-client";

const ROW =
  "flex min-h-14 w-full touch-manipulation items-center gap-3 rounded-lg border border-border bg-card px-4 py-2 text-left select-none active:bg-accent";

const FIELD =
  "h-12 w-full rounded-lg border border-input bg-card px-3 text-base text-foreground placeholder:text-muted-foreground/70 focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none";

/** Form "Pelanggan baru" (PR-02): nama, HP (wajib bila Bayar Nanti), tipe member; langsung terpilih. */
function NewCustomerForm({
  requirePhone,
  onCreated,
  onCancel,
}: {
  requirePhone: boolean;
  onCreated: (id: number) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [memberTypeId, setMemberTypeId] = useState<number | null>(null);
  const [memberTypes, setMemberTypes] = useState<PosMemberType[]>([]);
  const [errors, setErrors] = useState<{ name?: string; phone?: string }>({});
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  // Tipe member aktif; gagal muat (tanpa izin) = pilihan member disembunyikan.
  useEffect(() => {
    let alive = true;
    posService
      .memberTypes()
      .then((list) => {
        if (alive) setMemberTypes(list.filter((m) => m.status === "active"));
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  async function submit() {
    const input = { name, phone, memberTypeId, requirePhone };
    const errs = validateNewCustomer(input);
    setErrors(errs);
    if (errs.name || errs.phone) return;
    setSubmitting(true);
    setServerError(null);
    try {
      const created = await posService.createCustomer(newCustomerBody(input));
      onCreated(created.id);
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
      {serverError && <Notice tone="danger" title={serverError} />}
      <div>
        <label htmlFor="pos-new-name" className="mb-1.5 block text-sm font-medium text-foreground">
          Nama <span aria-hidden className="text-destructive">*</span>
        </label>
        <input id="pos-new-name" value={name} onChange={(e) => setName(e.target.value)} className={FIELD} autoComplete="off" />
        {errors.name && <p className="mt-1 text-xs font-medium text-destructive">{errors.name}</p>}
      </div>
      <div>
        <label htmlFor="pos-new-phone" className="mb-1.5 block text-sm font-medium text-foreground">
          Telepon / HP {requirePhone && <span aria-hidden className="text-destructive">*</span>}
        </label>
        <input
          id="pos-new-phone"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          inputMode="tel"
          className={FIELD}
          autoComplete="off"
        />
        {requirePhone && !errors.phone && (
          <p className="mt-1 text-xs text-muted-foreground">Wajib karena transaksi ini Bayar Nanti (termin net).</p>
        )}
        {errors.phone && <p className="mt-1 text-xs font-medium text-destructive">{errors.phone}</p>}
      </div>
      {memberTypes.length > 0 && (
        <div>
          <label htmlFor="pos-new-member" className="mb-1.5 block text-sm font-medium text-foreground">
            Tipe member
          </label>
          <select
            id="pos-new-member"
            value={memberTypeId ?? ""}
            onChange={(e) => setMemberTypeId(e.target.value === "" ? null : Number(e.target.value))}
            className={FIELD}
          >
            <option value="">Bukan member</option>
            {memberTypes.map((m) => (
              <option key={m.id} value={m.id}>
                {m.code} — {m.name}
              </option>
            ))}
          </select>
        </div>
      )}
      <div className="mt-auto flex gap-3 pt-2">
        <Button variant="secondary" className="h-14 flex-1 text-base" onClick={onCancel} disabled={submitting}>
          Batal
        </Button>
        <Button className="h-14 flex-1 text-base font-semibold" onClick={() => void submit()} loading={submitting}>
          Simpan &amp; pilih
        </Button>
      </div>
    </div>
  );
}

/** Sheet pemilih pelanggan: pencarian (debounce 350 ms), walk-in di urutan pertama, pelanggan baru. */
export function CustomerSheet({
  open,
  selectedId,
  requirePhone = false,
  onSelect,
  onClose,
}: {
  open: boolean;
  selectedId: number | null;
  /** Bayar Nanti: pelanggan baru wajib ber-telepon. */
  requirePhone?: boolean;
  onSelect: (id: number | null) => void;
  onClose: () => void;
}) {
  const can = useAuthStore((s) => s.can);
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<PosCustomer[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    const t = window.setTimeout(() => {
      setLoading(true);
      posService
        .searchCustomers(query)
        .then((res) => {
          if (!alive) return;
          setItems(res.items);
          setError(null);
        })
        .catch((err) => {
          if (!alive) return;
          setItems([]);
          setError(readApiError(err).message);
        })
        .finally(() => {
          if (alive) setLoading(false);
        });
    }, 350);
    return () => {
      alive = false;
      window.clearTimeout(t);
    };
  }, [open, query]);

  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent side="right" showCloseButton={false} className="flex w-full gap-0 bg-card p-0 sm:max-w-md">
        <SheetHeader className="flex-row items-center justify-between gap-3 border-b border-border p-4">
          <div className="min-w-0">
            <SheetTitle className="text-xl">{creating ? "Pelanggan baru" : "Pilih pelanggan"}</SheetTitle>
            <SheetDescription className="text-sm">
              {creating ? "Disimpan lalu langsung dipilih; harga member dihitung ulang." : "Kosongkan untuk walk-in (tanpa customer)."}
            </SheetDescription>
          </div>
          <button
            type="button"
            aria-label="Tutup"
            onClick={() => {
              setCreating(false);
              onClose();
            }}
            className="flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-muted-foreground select-none active:bg-accent"
          >
            <X aria-hidden className="size-6" />
          </button>
        </SheetHeader>
        {creating ? (
          <NewCustomerForm
            requirePhone={requirePhone}
            onCancel={() => setCreating(false)}
            onCreated={(id) => {
              setCreating(false);
              onSelect(id);
              onClose();
            }}
          />
        ) : (
          <>
        <div className="border-b border-border p-4">
          <div className="relative">
            <Search aria-hidden className="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-muted-foreground" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Ketik untuk mencari customer…"
              aria-label="Cari pelanggan"
              inputMode="search"
              autoComplete="off"
              className="h-12 w-full rounded-lg border border-input bg-card pr-3 pl-11 text-base text-foreground placeholder:text-muted-foreground/70 focus:border-ring focus:ring-[3px] focus:ring-ring/25 focus:outline-none"
            />
          </div>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto overscroll-contain p-4">
          {can("customers.create") && (
            <button type="button" onClick={() => setCreating(true)} className={cn(ROW, "border-dashed text-primary")}>
              <UserPlus aria-hidden className="size-5" />
              <span className="flex-1 text-base font-medium">Pelanggan baru</span>
            </button>
          )}
          <button
            type="button"
            onClick={() => {
              onSelect(null);
              onClose();
            }}
            className={cn(ROW, selectedId === null && "border-primary ring-1 ring-primary")}
          >
            <UserRound aria-hidden className="size-5 text-muted-foreground" />
            <span className="flex-1 text-base font-medium text-foreground">Walk-in (tanpa customer)</span>
            {selectedId === null && <Check aria-hidden className="size-5 text-primary" />}
          </button>
          {error && <Notice tone="warning" title={`Pelanggan tidak termuat: ${error}`} />}
          {loading && items.length === 0 && (
            <>
              <Skeleton className="h-14 rounded-lg" />
              <Skeleton className="h-14 rounded-lg" />
              <Skeleton className="h-14 rounded-lg" />
            </>
          )}
          {!loading && !error && items.length === 0 && (
            <p className="py-6 text-center text-sm text-muted-foreground">Tidak ada pelanggan yang cocok.</p>
          )}
          {items.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => {
                onSelect(c.id);
                onClose();
              }}
              className={cn(ROW, selectedId === c.id && "border-primary ring-1 ring-primary")}
            >
              <span className="min-w-0 flex-1">
                <span className="block truncate text-base font-medium text-foreground">
                  {c.code} — {c.name}
                </span>
                {c.phone && <span className="block text-xs text-muted-foreground">{c.phone}</span>}
              </span>
              {selectedId === c.id && <Check aria-hidden className="size-5 text-primary" />}
            </button>
          ))}
        </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
