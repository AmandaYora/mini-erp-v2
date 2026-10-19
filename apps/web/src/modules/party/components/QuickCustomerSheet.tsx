import { useState } from "react";
import type { SearchSelectOption } from "@/shared/components/ui";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { rowsToAttrs } from "@/shared/lib/key-value";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { PartyFormModal } from "@/modules/party/components/PartyFormModal";
import type { PartyFormValues } from "@/modules/party/schemas/party.schema";
import { customerService, memberTypeService } from "@/modules/party/services/party.service";
import type { Party } from "@/modules/party/types";

interface QuickCustomerSheetProps {
  open: boolean;
  /** Termin net: telepon wajib (aturan backend validateHeader). */
  requirePhone?: boolean;
  onClose: () => void;
  /** Dipanggil setelah customer tersimpan — pemanggil memilihnya ("Simpan & pilih"). */
  onCreated: (party: Party) => void;
}

/**
 * "Pelanggan baru" cepat untuk form transaksi (PR-01): membungkus PartyFormModal
 * yang sudah ada tanpa meninggalkan halaman, lalu menyerahkan hasilnya ke
 * pemanggil agar langsung terpilih. Form dipasang segar tiap dibuka.
 */
export function QuickCustomerSheet({ open, requirePhone = false, onClose, onCreated }: QuickCustomerSheetProps) {
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  // Tipe member aktif; gagal muat (tanpa izin) = field member disembunyikan.
  const { data: memberData } = useAsyncData(async () => {
    if (!open) return { options: [] as SearchSelectOption[], ok: false };
    try {
      const list = await memberTypeService.list("", "active");
      return { options: list.map((m) => ({ value: m.id, label: `${m.code} — ${m.name}` })), ok: true };
    } catch {
      return { options: [] as SearchSelectOption[], ok: false };
    }
  }, [open]);

  async function handleSubmit(v: PartyFormValues) {
    setServerError(null);
    if (requirePhone && v.phone.trim() === "") {
      setServerError("Termin net wajib memakai pelanggan yang memiliki nomor telepon.");
      return;
    }
    setSubmitting(true);
    try {
      const res = await customerService.create({
        code: v.code.trim() === "" ? undefined : v.code.trim(),
        name: v.name,
        phone: v.phone === "" ? undefined : v.phone,
        email: v.email === "" ? undefined : v.email,
        address: v.address === "" ? undefined : v.address,
        notes: v.notes === "" ? undefined : v.notes,
        memberTypeId: v.memberTypeId ?? undefined,
        attributes: rowsToAttrs(v.attrs),
      });
      toast.fromServer(res.message, "Pelanggan dibuat");
      onCreated(res.data);
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  if (!open) return null;
  return (
    <PartyFormModal
      open
      title="Pelanggan baru"
      description={requirePhone ? "Termin net: nomor telepon wajib diisi." : "Disimpan lalu langsung dipilih pada transaksi ini."}
      initial={null}
      withMember
      memberLoadOk={memberData?.ok ?? false}
      memberOptions={memberData?.options ?? []}
      submitting={submitting}
      serverError={serverError}
      onClose={onClose}
      onSubmit={(v) => void handleSubmit(v)}
    />
  );
}
