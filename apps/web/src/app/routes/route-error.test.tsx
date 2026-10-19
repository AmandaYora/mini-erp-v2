import { Suspense } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouteErrorBoundary } from "@/app/routes/RouteErrorBoundary";
import { CrashCard } from "@/shared/components/feedback/crash-card";
import { ErrorBoundary } from "@/shared/components/feedback/error-boundary";

function Boom(): never {
  throw new Error("render meledak");
}

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ErrorBoundary (OP-01)", () => {
  it("galat render menampilkan kartu Indonesia + id kesalahan + tombol muat ulang, bukan layar putih", () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.getByText("Terjadi kesalahan pada halaman ini")).toBeTruthy();
    expect(screen.getByText(/ID kesalahan: ERR-[A-Z0-9]+/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Muat ulang halaman" })).toBeTruthy();
  });

  it("anak sehat dirender normal", () => {
    render(
      <ErrorBoundary>
        <p>aman</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText("aman")).toBeTruthy();
  });

  it("galat chunk memakai pesan 'versi diperbarui'", () => {
    function Stale(): never {
      throw new TypeError("Failed to fetch dynamically imported module: /assets/x.js");
    }
    render(
      <ErrorBoundary>
        <Stale />
      </ErrorBoundary>,
    );
    expect(screen.getByText("Versi aplikasi sudah diperbarui")).toBeTruthy();
  });
});

describe("CrashCard", () => {
  it("tombol memanggil onReload", () => {
    const onReload = vi.fn();
    render(<CrashCard errorId="ERR-TEST01" onReload={onReload} />);
    fireEvent.click(screen.getByRole("button", { name: "Muat ulang halaman" }));
    expect(onReload).toHaveBeenCalledTimes(1);
    expect(screen.getByText("ID kesalahan: ERR-TEST01")).toBeTruthy();
  });
});

describe("errorElement rute (OP-01)", () => {
  it("galat di halaman anak tampil sebagai kartu DI DALAM layout (layout tetap ada)", async () => {
    const router = createMemoryRouter(
      [
        {
          path: "/",
          element: (
            <div>
              <nav>sidebar</nav>
              <Suspense fallback="muat">
                <Outlet />
              </Suspense>
            </div>
          ),
          children: [
            {
              errorElement: <RouteErrorBoundary />,
              children: [{ path: "rusak", element: <Boom /> }],
            },
          ],
        },
      ],
      { initialEntries: ["/rusak"] },
    );
    render(<RouterProvider router={router} />);
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByText("sidebar")).toBeTruthy();
    expect(screen.getByText(/ID kesalahan: ERR-/)).toBeTruthy();
  });
});
