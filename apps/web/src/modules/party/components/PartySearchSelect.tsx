import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { customerService, supplierService } from "@/modules/party/services/party.service";
import type { Party } from "@/modules/party/types";

export type PartyKind = "customer" | "supplier" | "any";

/** Pencarian pihak baru dikirim mulai 2 huruf (1 huruf terlalu lebar). */
const MIN_PARTY_SEARCH = 2;

type PartyOption = AsyncSearchOption & { party: Party };

export interface PartySearchSelectProps {
  id?: string;
  /** Pihak yang dipilih (id numerik). Null berarti belum memilih. */
  value: number | null;
  partyKind: PartyKind;
  onChange: (value: number | null, party?: Party) => void;
  /** Seed objek terpilih agar tampil instan tanpa fetch (mis. mode edit). */
  selectedParty?: Party | null;
  disabled?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
  limit?: number;
  /** Izinkan mengosongkan pilihan. Default true. */
  allowClear?: boolean;
  "aria-label"?: string;
}

function formatPartyOption(party: Party, withType = false): PartyOption {
  const contact = party.phone || party.email || party.code || undefined;
  const typeLabel = party.type === "supplier" ? "Supplier" : party.type === "customer" ? "Customer" : "";
  const memberSuffix = party.member?.code ? ` (${party.member.code})` : "";
  return {
    value: party.id,
    label: `${party.name}${memberSuffix}`,
    description: withType && typeLabel ? [typeLabel, contact].filter(Boolean).join(" · ") : contact,
    party,
  };
}

/**
 * PartySearchSelect — pemilih customer/supplier dengan pencarian SERVER-SIDE.
 *
 * Dibangun DI ATAS AsyncSearchSelect di dalam modul party (bukan di shared/
 * — shared tetap domain-agnostik). Tidak bergantung pada daftar halaman yang
 * dibatasi 50 baris, sehingga pihak ke-51+ tetap dapat dicari & dipilih.
 * Effect resolve hanya berisi callback async (tanpa setState sinkron di body)
 * sehingga bebas peringatan set-state-in-effect.
 */
export function PartySearchSelect({
  id,
  value,
  partyKind,
  onChange,
  selectedParty,
  disabled,
  placeholder,
  searchPlaceholder = "Cari nama, kode, atau kontak…",
  limit = 20,
  allowClear = true,
  "aria-label": ariaLabel,
}: PartySearchSelectProps) {
  const svc = partyKind === "supplier" ? supplierService : customerService;
  const [resolved, setResolved] = useState<PartyOption | null>(
    selectedParty ? formatPartyOption(selectedParty) : null,
  );
  // Seed/refresh dari props + pengosongan saat value dikosongkan — render-phase
  // adjustment (pola resmi React untuk derived state), bukan effect.
  const [prevSeed, setPrevSeed] = useState(selectedParty);
  if (selectedParty !== prevSeed) {
    setPrevSeed(selectedParty);
    if (selectedParty) setResolved(formatPartyOption(selectedParty));
  }
  if ((value === null || value === undefined) && resolved !== null) {
    setResolved(null);
  }

  const loadOptions = useCallback(
    async (search: string, signal?: AbortSignal): Promise<AsyncSearchLoadResult> => {
      const params = { search: search || undefined, status: "active", page: 1, limit };
      if (partyKind === "any") {
        // Pembayaran bisa ke customer maupun supplier: dua panggilan paralel.
        const half = Math.max(1, Math.ceil(limit / 2));
        const [c, s] = await Promise.all([
          customerService.list({ ...params, limit: half }, signal),
          supplierService.list({ ...params, limit: half }, signal),
        ]);
        return {
          options: [...c.items, ...s.items].map((p) => formatPartyOption(p, true)),
          hasMore: c.meta.total > c.items.length || s.meta.total > s.items.length,
        };
      }
      const res = await svc.list(params, signal);
      return {
        options: res.items.map((p) => formatPartyOption(p)),
        hasMore: res.meta.total > res.items.length,
      };
    },
    [svc, limit, partyKind],
  );

  // Pastikan pihak terpilih tampil walau di luar hasil pencarian saat ini.
  // Tanpa setState sinkron di body (hanya callback async) → bebas peringatan.
  useEffect(() => {
    if (!value || selectedParty?.id === value || resolved?.value === value) return;
    let active = true;
    const lookup: Promise<unknown> =
      partyKind === "any"
        ? customerService.get(value).catch(() => supplierService.get(value))
        : svc.get(value);
    lookup
      .then((party) => {
        if (active) setResolved(formatPartyOption(party as Party));
      })
      .catch(() => {
        if (active) setResolved(null);
      });
    return () => {
      active = false;
    };
  }, [value, selectedParty, resolved, svc, partyKind]);

  return (
    <AsyncSearchSelect
      id={id}
      aria-label={ariaLabel}
      value={value}
      disabled={disabled}
      minChars={MIN_PARTY_SEARCH}
      placeholder={
        placeholder ??
        (partyKind === "supplier"
          ? "Pilih supplier…"
          : partyKind === "any"
            ? "Pilih pelanggan / supplier…"
            : "Pilih customer…")
      }
      searchPlaceholder={searchPlaceholder}
      selectedOption={resolved}
      allowClear={allowClear}
      loadOptions={loadOptions}
      onChange={(next, option) => {
        const partyOption = option as PartyOption | undefined;
        setResolved(partyOption ?? null);
        onChange(
          typeof next === "number" ? next : next === null ? null : Number(next) || null,
          partyOption?.party,
        );
      }}
    />
  );
}
