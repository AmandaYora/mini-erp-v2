import { expect, type Page } from "@playwright/test";
import { session, type Role } from "./api";
import { L } from "./labels";

const ACCESS_KEY = "mini-erp.access-token";
const REFRESH_KEY = "mini-erp.refresh-token";

/** Masuk tanpa mengetik: token hasil REST ditanam ke localStorage sebelum halaman dimuat. */
export async function loginAs(page: Page, role: Role) {
  const s = await session(role);
  await page.addInitScript(([a, r, ak, rk]) => {
    try {
      if (!localStorage.getItem(ak)) {
        localStorage.setItem(ak, a);
        localStorage.setItem(rk, r);
      }
    } catch {
      /* ignore */
    }
  }, [s.access, s.refresh, ACCESS_KEY, REFRESH_KEY]);
}

/** Login lewat formulir (dipakai E01). */
export async function uiLogin(page: Page, username: string, password: string) {
  await page.goto("/login");
  await page.getByLabel(L.login.user).fill(username);
  await page.getByLabel(L.login.pass, { exact: true }).fill(password);
  await page.getByRole("button", { name: L.login.submit }).click();
}

export async function expectNoRuntimeErrors(page: Page, errors: string[]) {
  expect(errors, `error runtime di ${page.url()}`).toEqual([]);
}

export function collectErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  return errors;
}

export const rupiah = (n: number) => new Intl.NumberFormat("id-ID").format(n);
