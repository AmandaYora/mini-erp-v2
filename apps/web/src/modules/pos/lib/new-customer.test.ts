import { describe, expect, it } from "vitest";
import { newCustomerBody, validateNewCustomer } from "@/modules/pos/lib/new-customer";

const base = { name: "Siti", phone: "", memberTypeId: null, requirePhone: false };

describe("pelanggan baru di POS (PR-02)", () => {
  it("nama wajib; telepon opsional bila bukan Bayar Nanti", () => {
    expect(validateNewCustomer({ ...base, name: " " }).name).toBeDefined();
    expect(validateNewCustomer(base)).toEqual({});
  });
  it("Bayar Nanti menegakkan telepon wajib", () => {
    expect(validateNewCustomer({ ...base, requirePhone: true }).phone).toMatch(/wajib/);
    expect(validateNewCustomer({ ...base, requirePhone: true, phone: "0812345678" })).toEqual({});
  });
  it("format telepon mengikuti backend", () => {
    expect(validateNewCustomer({ ...base, phone: "12ab" }).phone).toBeDefined();
    expect(validateNewCustomer({ ...base, phone: "123" }).phone).toBeDefined();
    expect(validateNewCustomer({ ...base, phone: "+62 812-3456-78" })).toEqual({});
  });
  it("body hanya memuat isian yang terisi", () => {
    expect(newCustomerBody(base)).toEqual({ name: "Siti" });
    expect(newCustomerBody({ ...base, phone: " 0812345678 ", memberTypeId: 4 })).toEqual({
      name: "Siti",
      phone: "0812345678",
      memberTypeId: 4,
    });
  });
});
