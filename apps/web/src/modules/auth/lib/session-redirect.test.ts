import { describe, expect, it } from "vitest";
import { loginRedirectTarget, safeNextPath } from "@/modules/auth/lib/session-redirect";

describe("loginRedirectTarget", () => {
  it("membawa path dan query", () => {
    expect(loginRedirectTarget("/sales-orders/new", "?a=1")).toBe(
      `/login?next=${encodeURIComponent("/sales-orders/new?a=1")}`,
    );
  });
  it("halaman publik tidak diredirect", () => {
    expect(loginRedirectTarget("/login", "")).toBeNull();
    expect(loginRedirectTarget("/403", "")).toBeNull();
  });
});

describe("safeNextPath", () => {
  it("hanya path internal", () => {
    expect(safeNextPath("/pos", "/dashboard")).toBe("/pos");
    expect(safeNextPath("https://evil.example", "/dashboard")).toBe("/dashboard");
    expect(safeNextPath("//evil.example", "/dashboard")).toBe("/dashboard");
    expect(safeNextPath(null, "/dashboard")).toBe("/dashboard");
  });
});
