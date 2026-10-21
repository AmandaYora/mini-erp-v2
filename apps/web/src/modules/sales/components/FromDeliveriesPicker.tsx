import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, Checkbox, ErrorState, FormField, Modal, TableSkeleton } from "@/shared/components/ui";
import { DocNumber } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import type { Party } from "@/modules/party/types";
import { salesService } from "@/modules/sales/services/sales.service";
import { ageDays, fromDeliveriesPath, MAX_FROM_DELIVERIES } from "@/modules/sales/lib/from-deliveries";

/** Batas tampil per pelanggan; server membatasi 1–20 SJ per order. */
const PICKER_LIMIT = 50;

export interface FromDeliveriesPickerProps {
  open: boolean;
  onClose: () => void;
  /** Pelanggan awal (mis. dari form order biasa); null = pilih dulu. */
  initialPartyId?: number | null;
}

/**
 * Pintu masuk "Dari surat jalan" (ADR-0022 W-05/W-07): pilih pelanggan → centang SJ Langsung
 * yang belum diorder → lanjut ke form order mode "dari SJ" (`/sales-orders/new?fromDeliveries=`).
 */
export function FromDeliveriesPicker({ open, onClose, initialPartyId = null }: FromDeliveriesPickerProps) {
  const navigate = useNavigate();
  const [partyId, setPartyId] = useState<number | null>(initialPartyId);
  const [pickedParty, setPickedParty] = useState<Party | null>(null);
  const [selected, setSelected] = useState<Set<number>>(() => new Set());

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      !open || partyId === null
        ? Promise.resolve(null)
        : salesService.openDirectDeliveries(partyId, PICKER_LIMIT, signal),
    [open, partyId],
  );
  const notes = data?.items ?? [];
  const today = todayWIB();
  const chosen = notes.filter((n) => selected.has(n.id)).map((n) => n.id);
  const tooMany = chosen.length > MAX_FROM_DELIVERIES;

  function choose(id: number | null, party?: Party) {
    setPartyId(id);
    setPickedParty(party ?? null);
    setSelected(new Set());
  }

  function go() {
    if (chosen.length === 0 || tooMany) return;
    onClose();
    navigate(fromDeliveriesPath(chosen));
  }

  return (
    <Modal
      open={open}
      title="Order dari surat jalan"
      description="Pilih pelanggan, lalu centang surat jalan langsung yang akan dijadikan satu order."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>
            Batal
          </Button>
          <Button size="sm" onClick={go} disabled={chosen.length === 0 || tooMany}>
            {chosen.length > 1 ? `Buat order dari ${chosen.length} SJ` : "Buat order"}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <FormField label="Pelanggan" required>
          <PartySearchSelect
            partyKind="customer"
            value={partyId}
            selectedParty={pickedParty}
            onChange={choose}
            placeholder="Cari pelanggan (nama, kode, telepon)…"
            aria-label="Pelanggan surat jalan"
          />
        </FormField>

        {partyId === null ? (
          <p className="text-sm text-muted-foreground">Pilih pelanggan untuk melihat surat jalannya yang belum diorder.</p>
        ) : loading ? (
          <TableSkeleton rows={3} />
        ) : error ? (
          <ErrorState title="Gagal memuat surat jalan" message={error} onRetry={reload} />
        ) : notes.length === 0 ? (
          <EmptyState
            title="Tidak ada surat jalan menunggu order"
            description="Pelanggan ini tidak punya SJ Langsung terkonfirmasi yang belum diorder."
          />
        ) : (
          <ul className="flex flex-col divide-y divide-border rounded-md border border-border" aria-label="Surat jalan menunggu order">
            {notes.map((n) => {
              const age = ageDays(n.deliveryDate, today);
              const checked = selected.has(n.id);
              return (
                <li key={n.id}>
                  <label className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm">
                    <Checkbox
                      checked={checked}
                      aria-label={`Pilih ${n.number}`}
                      onCheckedChange={(v) =>
                        setSelected((prev) => {
                          const next = new Set(prev);
                          if (v === true) next.add(n.id);
                          else next.delete(n.id);
                          return next;
                        })
                      }
                    />
                    <span className="min-w-0 flex-1">
                      <DocNumber value={n.number} />
                      <span className="ml-2 text-muted-foreground">
                        {formatDate(n.deliveryDate)} · {age} hari · {n.items.length} baris
                      </span>
                    </span>
                  </label>
                </li>
              );
            })}
          </ul>
        )}
        {tooMany && (
          <p className="text-xs font-medium text-destructive">Maksimal {MAX_FROM_DELIVERIES} surat jalan per order.</p>
        )}
      </div>
    </Modal>
  );
}
