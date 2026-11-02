import { useEffect, useState } from "react";
import { financeService } from "@/modules/finance/services/finance.service";
import type { CashAccountOption } from "@/modules/finance/types";
import { activeCashAccounts, cashAccountChoiceShown } from "@/modules/finance/lib/cash-accounts";

// Daftar akun kas untuk pembayaran tunai. Satu permintaan dibagi semua komponen
// di halaman yang sama (cache janji modul), sama seperti use-bank-accounts.

let cache: Promise<CashAccountOption[]> | null = null;

function load(): Promise<CashAccountOption[]> {
  if (!cache) {
    cache = financeService
      .cashAccountOptions()
      .then((list) => list ?? [])
      .catch(() => {
        cache = null; // gagal: pemilih tidak tampil, tunai ke Kas Tunai seperti sebelumnya
        return [];
      });
  }
  return cache;
}

export function invalidateCashAccounts() {
  cache = null;
}

export interface CashAccountsState {
  /** Semua kas, termasuk yang diarsipkan (label pembayaran lama). */
  all: CashAccountOption[];
  /** Kas aktif — pilihan untuk pembayaran baru. */
  active: CashAccountOption[];
  /** true bila ada lebih dari satu kas aktif: pembayaran tunai memilih kasnya. */
  shown: boolean;
  loading: boolean;
}

export function useCashAccounts(): CashAccountsState {
  const [all, setAll] = useState<CashAccountOption[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let alive = true;
    void load().then((list) => {
      if (!alive) return;
      setAll(list);
      setLoading(false);
    });
    return () => {
      alive = false;
    };
  }, []);
  return { all, active: activeCashAccounts(all), shown: cashAccountChoiceShown(all), loading };
}
