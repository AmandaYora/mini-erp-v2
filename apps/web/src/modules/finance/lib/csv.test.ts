import { describe, expect, it } from "vitest";
import { buildCsv } from "@/modules/finance/lib/csv";

describe("buildCsv", () => {
  it("menulis baris dengan pemisah koma dan CRLF", () => {
    expect(buildCsv([["Kode", "Debit"], ["1000", 250000]])).toBe("Kode,Debit\r\n1000,250000");
  });

  it("mengutip sel berkoma, bertanda kutip, atau berbaris baru", () => {
    expect(buildCsv([['Kas, tunai', 'Ia "baik"', "a\nb"]])).toBe('"Kas, tunai","Ia ""baik""","a\nb"');
  });

  it("sel kosong menjadi string kosong", () => {
    expect(buildCsv([[null, undefined, 0]])).toBe(",,0");
  });
});
