import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { RequireAuth } from "@/app/routes/guards";
import NotFoundPage from "@/modules/auth/pages/NotFoundPage";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<div>halaman login</div>} />
        <Route
          element={
            <RequireAuth>
              <div>layout</div>
            </RequireAuth>
          }
        >
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  useAuthStore.setState({ ready: true, user: null, bootError: null });
});
afterEach(() => cleanup());

describe("rute tak dikenal", () => {
  it("pengguna login lolos guard ke layout", () => {
    useAuthStore.setState({
      user: { id: 1, username: "a", fullName: "A", roleIds: [] },
    });
    renderAt("/tidak-ada/sama-sekali");
    // RequireAuth merender anak langsung (layout palsu), jadi uji komponennya.
    expect(screen.getByText("layout")).toBeTruthy();
  });

  it("NotFoundPage menampilkan path dan tautan Dashboard", () => {
    render(
      <MemoryRouter initialEntries={["/foo/bar"]}>
        <Routes>
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText("404")).toBeTruthy();
    expect(screen.getByText(/\/foo\/bar/)).toBeTruthy();
    expect(screen.getByText("Kembali ke Dashboard").getAttribute("href")).toBe("/dashboard");
  });

  it("tamu diarahkan ke login (bukan 404)", () => {
    renderAt("/tidak-ada?x=1");
    expect(screen.getByText("halaman login")).toBeTruthy();
  });

  it("server tak terjangkau saat boot: layar coba lagi, bukan login", () => {
    useAuthStore.setState({ user: null, bootError: "network" });
    renderAt("/dashboard");
    expect(screen.getByText("Tidak dapat terhubung ke server")).toBeTruthy();
    expect(screen.queryByText("halaman login")).toBeNull();
  });
});
