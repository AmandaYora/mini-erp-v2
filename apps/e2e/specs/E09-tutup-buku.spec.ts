import { test, expect } from "@playwright/test";
import { call, ok, session, today } from "../helpers/api";
import { BASE_URL } from "../helpers/env";
import { loginAs } from "../helpers/ui";

const now = new Date(new Date().toLocaleString("en-US", { timeZone: "Asia/Jakarta" }));
const year = now.getFullYear();
const month = now.getMonth() + 1;

test.describe.serial("E09 tutup buku", () => {
  test("kesiapan -> tutup aman -> posting diblokir -> buka ulang", async () => {
    await ok("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const ready = await ok<any>("akuntan", "GET", `/finance/close/readiness?year=${year}&month=${month}`);
    expect(ready.checks.length).toBeGreaterThan(0);
    expect(ready).toBeTruthy();

    const closed = await call("akuntan", "POST", "/finance/close/safe-close", { year, month });
    expect(closed.status, closed.message + JSON.stringify(closed.errors)).toBeLessThan(300);
    const periods = await ok<any[]>("akuntan", "GET", "/finance/periods");
    expect(periods.find((p) => p.year === year && p.month === month)?.status).toBe("closed");

    // jurnal manual pada periode tertutup ditolak
    const accounts = await ok<any[]>("akuntan", "GET", "/finance/accounts");
    const [a, b] = [accounts[0], accounts[1]];
    const manualBody = {
      date: today(), memo: "uji periode tertutup",
      lines: [{ accountCode: a.code, debit: 1000, credit: 0 }, { accountCode: b.code, debit: 0, credit: 1000 }],
    };
    const manual = await call("akuntan", "POST", "/finance/journals/manual", manualBody);
    expect(manual.status).toBe(409);
    expect(manual.message).toMatch(/ditutup/);

    await ok("akuntan", "POST", "/finance/periods/reopen", { year, month, reason: "uji e2e buka ulang" });
    const after = await ok<any[]>("akuntan", "GET", "/finance/periods");
    expect(after.find((p) => p.year === year && p.month === month)?.status ?? "open").not.toBe("closed");
    // setelah dibuka ulang, jurnal manual yang sama diterima
    const again = await call("akuntan", "POST", "/finance/journals/manual", manualBody);
    expect(again.status, again.message + JSON.stringify(again.errors)).toBeLessThan(300);
  });

  test("periode pajak: tutup lalu buka ulang", async () => {
    const c = await call("akuntan", "POST", "/finance/tax-periods/close", { year, month });
    expect(c.status, c.message).toBeLessThan(300);
    const list = await ok<any[]>("akuntan", "GET", "/finance/tax-periods");
    expect(list.find((p) => p.year === year && p.month === month)?.status).toBe("closed");
    await ok("akuntan", "POST", "/finance/tax-periods/reopen", { year, month, reason: "uji e2e buka ulang pajak" });
  });

  test("paket pajak terunduh (xlsx/zip tidak kosong)", async () => {
    const s = await session("akuntan");
    const res = await fetch(`${BASE_URL}/api/v1/finance/export/tax-package?year=${year}&month=${month}`, {
      headers: { Authorization: `Bearer ${s.access}` },
    });
    expect(res.status).toBe(200);
    const buf = Buffer.from(await res.arrayBuffer());
    expect(buf.length).toBeGreaterThan(200);
    expect(buf.subarray(0, 2).toString()).toBe("PK"); // zip/xlsx
  });

  test("UI: halaman Periode memuat status periode berjalan", async ({ page }) => {
    await loginAs(page, "akuntan");
    await page.goto("/finance/periods");
    await expect(page.getByRole("heading", { level: 1, name: "Periode" })).toBeVisible();
    await expect(page.getByText(/Terbuka|Terkunci/).first()).toBeVisible();
  });
});
