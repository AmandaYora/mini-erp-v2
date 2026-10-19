import { useCallback, useRef, useState } from "react";
import { emptyDraftLine } from "@/modules/finance/lib/journal-lines";
import type { DraftLine } from "@/modules/finance/lib/journal-lines";

/** State baris editor jurnal (min. dua baris kosong): ubah, tambah, hapus, kosongkan. */
export function useJournalDraft(initialCount = 2) {
  const next = useRef(initialCount + 1);
  const [lines, setLines] = useState<DraftLine[]>(() =>
    Array.from({ length: initialCount }, (_, i) => emptyDraftLine(i + 1)),
  );

  const update = useCallback((key: number, patch: Partial<DraftLine>) => {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)));
  }, []);
  const add = useCallback(() => {
    setLines((prev) => [...prev, emptyDraftLine(next.current++)]);
  }, []);
  const remove = useCallback((index: number) => {
    setLines((prev) => (prev.length <= 2 ? prev : prev.filter((_, i) => i !== index)));
  }, []);
  const reset = useCallback(() => {
    setLines([emptyDraftLine(next.current++), emptyDraftLine(next.current++)]);
  }, []);

  return { lines, update, add, remove, reset };
}

export type JournalDraft = ReturnType<typeof useJournalDraft>;
