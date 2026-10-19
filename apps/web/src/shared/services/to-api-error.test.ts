import { describe, expect, it } from "vitest";

import { toApiError } from "./http-client";

describe("toApiError", () => {
  it("mengembalikan ApiError apa adanya (pesan server tidak hilang)", () => {
    const err = { status: 409, message: "Stok tidak mencukupi", errors: [{ field: "qty", message: "x" }] };
    expect(toApiError(err)).toBe(err);
  });

  it("memberi pesan generik untuk nilai tak dikenal", () => {
    expect(toApiError(new Error("boom"))).toEqual({ status: 0, message: "Terjadi kesalahan tak terduga" });
    expect(toApiError(null).status).toBe(0);
  });
});
