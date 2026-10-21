import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation, useNavigate } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { DataTable } from "@/shared/components/ui/data-table";
import { readSavedViews } from "@/shared/lib/list-views";
import { setPrefsOwner } from "@/shared/stores/ui-prefs.store";
import { SavedViews } from "./saved-views";

type Row = { n: string };
const columns = [
  { header: "Nama", render: (r: Row) => r.n },
  { header: "Kanal", render: () => "POS" },
];

function Harness() {
  const loc = useLocation();
  const navigate = useNavigate();
  return (
    <>
      <SavedViews listKey="uji" columnsKey="uji" />
      <DataTable<Row> columns={columns} rows={[{ n: "A" }]} rowKey={(r) => r.n} columnsKey="uji" />
      <button type="button" onClick={() => navigate("/daftar?status=completed")}>
        Ubah filter
      </button>
      <div data-testid="search">{loc.search}</div>
    </>
  );
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Harness />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  localStorage.clear();
  setPrefsOwner(9);
});
afterEach(() => {
  cleanup();
  setPrefsOwner(null);
});

describe("SavedViews", () => {
  it("menyimpan filter + kolom, lalu menerapkannya kembali", async () => {
    const user = userEvent.setup();
    renderAt("/daftar?status=draft&page=4");

    // Sembunyikan kolom Kanal lewat menu Kolom tabel.
    await user.click(screen.getByRole("button", { name: "Kolom" }));
    await user.click(screen.getByRole("checkbox", { name: "Kanal" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("columnheader", { name: "Kanal" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Tampilan" }));
    await user.type(screen.getByLabelText("Simpan filter dan kolom saat ini"), "Draf saja");
    await user.click(screen.getByRole("button", { name: "Simpan tampilan" }));
    expect(readSavedViews("uji")).toEqual([
      expect.objectContaining({ name: "Draf saja", query: "status=draft", hiddenColumns: ["Kanal"] }),
    ]);
    // Tampilan yang cocok dengan keadaan sekarang menjadi label tombol.
    expect(screen.getByRole("button", { name: "Draf saja" })).toBeInTheDocument();

    // Ubah filter dan tampilkan lagi kolomnya, lalu terapkan tampilan tersimpan.
    await user.click(screen.getByRole("button", { name: "Ubah filter" }));
    await user.click(screen.getByRole("button", { name: "Kolom" }));
    await user.click(screen.getByRole("checkbox", { name: "Kanal" }));
    await user.keyboard("{Escape}");
    expect(screen.getByRole("columnheader", { name: "Kanal" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Tampilan" }));
    const list = screen.getByRole("list", { name: "Tampilan tersimpan" });
    await user.click(within(list).getByRole("button", { name: "Draf saja" }));
    expect(screen.getByTestId("search")).toHaveTextContent("?status=draft");
    expect(screen.queryByRole("columnheader", { name: "Kanal" })).not.toBeInTheDocument();
  });

  it("tampilan bawaan mengosongkan filter dan kolom; nama kosong ditolak dengan pesan", async () => {
    const user = userEvent.setup();
    renderAt("/daftar?status=draft");

    await user.click(screen.getByRole("button", { name: "Tampilan" }));
    await user.click(screen.getByRole("button", { name: "Simpan tampilan" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Nama tampilan wajib diisi.");

    await user.click(screen.getByRole("button", { name: "Kembali ke tampilan bawaan" }));
    expect(screen.getByTestId("search")).toBeEmptyDOMElement();
  });

  it("hapus tampilan", async () => {
    const user = userEvent.setup();
    localStorage.setItem(
      "mini-erp.views.uji@9",
      JSON.stringify([{ id: "x", name: "Lama", query: "status=draft", hiddenColumns: null }]),
    );
    renderAt("/daftar");
    await user.click(screen.getByRole("button", { name: "Tampilan" }));
    await user.click(screen.getByRole("button", { name: "Hapus tampilan Lama" }));
    expect(readSavedViews("uji")).toEqual([]);
    expect(screen.getByText(/Belum ada/)).toBeInTheDocument();
  });
});
