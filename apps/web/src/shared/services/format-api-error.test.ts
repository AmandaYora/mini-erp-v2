import { AxiosError, AxiosHeaders } from "axios";
import { describe, expect, it } from "vitest";

import { fieldErrorMessages, formatApiError } from "./http-client";

function axios422(data: unknown): AxiosError {
  const headers = new AxiosHeaders();
  return new AxiosError("Request failed", "ERR_BAD_REQUEST", { headers }, null, {
    status: 422,
    statusText: "Unprocessable Entity",
    headers,
    config: { headers },
    data,
  });
}

describe("formatApiError", () => {
  it("menyertakan pesan per-field setelah message server", () => {
    const err = {
      status: 422,
      message: "Validasi gagal",
      errors: [{ field: "paymentTerms", message: "customer termin net wajib memiliki nomor telepon" }],
    };
    expect(formatApiError(err)).toBe("Validasi gagal — customer termin net wajib memiliki nomor telepon");
  });

  it("membaca errors[] langsung dari AxiosError", () => {
    const err = axios422({
      success: false,
      message: "Validasi gagal",
      errors: [
        { field: "items[0].qty", message: "melebihi sisa yang bisa diretur" },
        { field: "reason", message: "wajib diisi" },
      ],
    });
    expect(formatApiError(err)).toBe("Validasi gagal — melebihi sisa yang bisa diretur; wajib diisi");
  });

  it("tanpa errors[] = message apa adanya", () => {
    expect(formatApiError({ status: 409, message: "Stok tidak mencukupi" })).toBe("Stok tidak mencukupi");
    expect(formatApiError(new Error("boom"))).toBe("Terjadi kesalahan tak terduga");
  });

  it("membuang pesan kosong, ganda, dan yang sama dengan message", () => {
    const err = {
      status: 400,
      message: "Validasi gagal",
      errors: [
        { field: "a", message: "wajib diisi" },
        { field: "b", message: "wajib diisi" },
        { field: "c", message: " " },
        { field: "d", message: "Validasi gagal" },
      ],
    };
    expect(fieldErrorMessages(err)).toEqual(["wajib diisi"]);
    expect(formatApiError(err)).toBe("Validasi gagal — wajib diisi");
  });
});
