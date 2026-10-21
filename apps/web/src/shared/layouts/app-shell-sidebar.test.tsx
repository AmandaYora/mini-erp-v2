import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import AppShell from "./AppShell";

const KEY = "mini-erp.sidebar.collapsed";

function renderShell() {
  return render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/dashboard" element={<div>isi</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

function sidebarState(container: HTMLElement) {
  return container.querySelector('[data-slot="sidebar"][data-state]')?.getAttribute("data-state");
}

beforeEach(() => {
  localStorage.removeItem(KEY);
  useAuthStore.setState({ permissions: [], ready: true });
});
afterEach(cleanup);

describe("AppShell — collapse sidebar", () => {
  it("tombol menu menutup lalu membuka sidebar dan mengingat pilihannya", async () => {
    const user = userEvent.setup();
    const { container } = renderShell();
    expect(sidebarState(container)).toBe("expanded");

    await user.click(screen.getByRole("button", { name: "Buka/tutup menu" }));
    expect(sidebarState(container)).toBe("collapsed");
    expect(localStorage.getItem(KEY)).toBe("1");

    await user.click(screen.getByRole("button", { name: "Buka/tutup menu" }));
    expect(sidebarState(container)).toBe("expanded");
    expect(localStorage.getItem(KEY)).toBe("0");
  });

  it("memulai dalam keadaan tertutup bila terakhir ditutup", () => {
    localStorage.setItem(KEY, "1");
    const { container } = renderShell();
    expect(sidebarState(container)).toBe("collapsed");
  });
});
