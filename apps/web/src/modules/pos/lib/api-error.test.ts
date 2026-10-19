import { describe, expect, it } from "vitest";
import { isDefiniteRejection, readApiError } from "@/modules/pos/lib/api-error";

describe("readApiError", () => {
  it("mempertahankan status + pesan server dari ApiError interceptor", () => {
    expect(readApiError({ status: 409, message: "clientRef sama, isi beda" })).toMatchObject({
      status: 409,
      message: "clientRef sama, isi beda",
    });
  });
  it("jatuh ke pesan generik untuk error asing", () => {
    expect(readApiError(new Error("x")).status).toBe(0);
  });
});

describe("isDefiniteRejection", () => {
  it("4xx pasti ditolak; jaringan putus / 5xx ambigu", () => {
    expect(isDefiniteRejection({ status: 422, message: "m" })).toBe(true);
    expect(isDefiniteRejection({ status: 0, message: "m" })).toBe(false);
    expect(isDefiniteRejection({ status: 503, message: "m" })).toBe(false);
  });
});
