import { describe, expect, it } from "vitest";
import { documentBankAccounts, printedBankAccounts, transferToLine, transferToListLine } from "@/modules/print/document";

const bca = { accountId: 11, bankName: "BCA", accountNumber: "123", accountHolder: "PT Uji", showOnDocuments: true, status: "active" };
const hidden = { ...bca, accountId: 12, bankName: "Mandiri", showOnDocuments: false };
const archived = { ...bca, accountId: 13, bankName: "BRI", status: "archived" };

describe("rekening di dokumen cetak (ADR-0026)", () => {
  it("daftar rekening dokumen = master aktif yang ditandai tampil", () => {
    expect(documentBankAccounts([bca, hidden, archived])).toEqual([{ bank: "BCA", holder: "PT Uji", number: "123" }]);
  });

  it("master satu-satunya sumber begitu berisi; teks lama hanya saat master kosong", () => {
    const legacy = [{ bank: "BNI", holder: "Toko Lama", number: "999" }];
    expect(printedBankAccounts([], legacy)).toEqual(legacy);
    expect(printedBankAccounts([bca, hidden], legacy)).toEqual([{ bank: "BCA", holder: "PT Uji", number: "123" }]);
    // Semua rekening master diarsipkan/disembunyikan: tidak kembali ke teks lama yang mungkin usang.
    expect(printedBankAccounts([hidden, archived], legacy)).toEqual([]);
  });

  it("baris transfer nota = daftar rekening dokumen", () => {
    expect(transferToListLine([{ bank: "BCA", holder: "PT Uji", number: "123" }, { bank: "BNI", holder: "", number: "9" }])).toBe(
      "Transfer ke: BCA 123 a.n. PT Uji / BNI 9",
    );
    expect(transferToListLine([{ bank: "BCA", holder: "", number: " " }])).toBe("");
  });

  it("baris transfer ke rekening satu pembayaran", () => {
    expect(transferToLine([bca, hidden], 12)).toBe("Transfer ke: Mandiri 123 a.n. PT Uji");
    expect(transferToLine([bca], 11, "")).toBe("BCA 123 a.n. PT Uji");
    // Rekening yang sudah diarsipkan tetap tercetak di dokumen lamanya.
    expect(transferToLine([archived], 13)).toBe("Transfer ke: BRI 123 a.n. PT Uji");
    expect(transferToLine([bca], 0)).toBe("");
    expect(transferToLine(undefined, 11)).toBe("");
  });
});
