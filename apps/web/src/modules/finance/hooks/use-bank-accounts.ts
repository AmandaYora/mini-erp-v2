import { useEffect, useState } from "react";
import { financeService } from "@/modules/finance/services/finance.service";
import type { BankAccount } from "@/modules/finance/types";
import { activeBankAccounts, defaultBankAccountId } from "@/modules/finance/lib/bank-accounts";

// Daftar rekening perusahaan (ADR-0026) untuk pemilih dan cetak. Satu
// permintaan dibagi semua komponen di halaman yang sama (cache janji modul);
// invalidateBankAccounts() dipanggil setelah admin mengubah daftar.

let cache: Promise<BankAccount[]> | null = null;

function load(): Promise<BankAccount[]> {
  if (!cache) {
    cache = financeService
      .bankAccountOptions()
      .then((list) => list ?? [])
      .catch(() => {
        cache = null; // gagal: coba lagi pada pemakaian berikutnya; pemilih tampil kosong
        return [];
      });
  }
  return cache;
}

export function invalidateBankAccounts() {
  cache = null;
}

export interface BankAccountsState {
  /** Semua rekening, termasuk yang diarsipkan (label dokumen lama). */
  all: BankAccount[];
  /** Rekening aktif — pilihan untuk tagihan/pembayaran baru. */
  active: BankAccount[];
  /** Akun buku besar rekening default, null bila tidak ada. */
  defaultAccountId: number | null;
  /** true bila perusahaan sudah memakai rekening: transfer/cek wajib memilih. */
  required: boolean;
  loading: boolean;
}

export function useBankAccounts(): BankAccountsState {
  const [all, setAll] = useState<BankAccount[]>([]);
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
  const active = activeBankAccounts(all);
  return { all, active, defaultAccountId: defaultBankAccountId(all), required: active.length > 0, loading };
}
