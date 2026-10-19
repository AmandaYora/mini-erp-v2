import { useState } from "react";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ConfirmDialog, DataTable, Modal, SearchSelect, StatusBadge } from "@/shared/components/ui";
import { DocNumber, DocumentRail, LineItemsEditor, Money } from "@/shared/components/patterns";

afterEach(cleanup);

describe("DataTable", () => {
  const cols = [{ header: "Nama", render: (r: { n: string }) => r.n, sortKey: "n" }];

  it("rows null diperlakukan kosong (Go mengirim null untuk slice kosong)", () => {
    render(<DataTable columns={cols} rows={null} rowKey={(r) => r.n} emptyTitle="Kosong" />);
    expect(screen.getByText("Kosong")).toBeInTheDocument();
  });

  it("menampilkan galat API asli + Coba lagi", async () => {
    const onRetry = vi.fn();
    render(<DataTable columns={cols} rows={[]} rowKey={(r: { n: string }) => r.n} error="Server menolak" onRetry={onRetry} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Server menolak");
    await userEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(onRetry).toHaveBeenCalled();
  });

  it("skeleton saat memuat tanpa baris", () => {
    render(<DataTable columns={cols} rows={[]} rowKey={(r: { n: string }) => r.n} loading />);
    expect(screen.getByRole("status", { name: "Memuat data" })).toBeInTheDocument();
  });

  it("sortir lewat header dan klik baris (mengabaikan klik pada tautan)", async () => {
    const onSort = vi.fn();
    const onRow = vi.fn();
    render(
      <DataTable
        columns={[
          { header: "Nama", render: (r: { n: string }) => r.n, sortKey: "n" },
          { header: "Aksi", render: () => <button type="button">Hapus</button> },
        ]}
        rows={[{ n: "A" }, { n: "B" }]}
        rowKey={(r) => r.n}
        sort={{ key: "n", dir: "asc" }}
        onSortChange={onSort}
        onRowClick={onRow}
      />,
    );
    expect(screen.getByRole("columnheader", { name: /Nama/ })).toHaveAttribute("aria-sort", "ascending");
    await userEvent.click(screen.getByRole("button", { name: /Nama/ }));
    expect(onSort).toHaveBeenCalledWith({ key: "n", dir: "desc" });
    await userEvent.click(screen.getAllByRole("button", { name: "Hapus" })[0]);
    expect(onRow).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("B"));
    expect(onRow).toHaveBeenCalledWith({ n: "B" });
  });

  it("seleksi + aksi massal", async () => {
    function Harness() {
      const [sel, setSel] = useState<Set<string | number>>(new Set());
      return (
        <DataTable
          columns={cols}
          rows={[{ n: "A" }, { n: "B" }]}
          rowKey={(r) => r.n}
          selectable
          selectedKeys={sel}
          onSelectionChange={setSel}
          columnsKey="test"
          bulkActions={(k) => <span>Aksi untuk {k.size}</span>}
        />
      );
    }
    render(<Harness />);
    await userEvent.click(screen.getByRole("checkbox", { name: "Pilih semua baris" }));
    expect(screen.getByText("2 dipilih")).toBeInTheDocument();
    expect(screen.getByText("Aksi untuk 2")).toBeInTheDocument();
  });
});

describe("LineItemsEditor", () => {
  it("Enter di baris terakhir menambah baris; hapus memanggil onRemove", async () => {
    const onAdd = vi.fn();
    const onRemove = vi.fn();
    render(
      <LineItemsEditor
        lines={[{ id: 1 }, { id: 2 }]}
        getKey={(l) => l.id}
        columns={[{ id: "qty", header: "Qty", render: (_l, i) => <input aria-label={`Qty ${i + 1}`} /> }]}
        onAdd={onAdd}
        onRemove={onRemove}
      />,
    );
    await userEvent.type(screen.getByLabelText("Qty 1"), "{Enter}");
    expect(onAdd).not.toHaveBeenCalled();
    await userEvent.type(screen.getByLabelText("Qty 2"), "{Enter}");
    expect(onAdd).toHaveBeenCalledTimes(1);
    await userEvent.click(screen.getByRole("button", { name: "Hapus baris 1" }));
    expect(onRemove).toHaveBeenCalledWith(0);
  });

  it("readOnly: tanpa tombol tambah/hapus", () => {
    render(
      <LineItemsEditor readOnly lines={[{ id: 1 }]} getKey={(l) => l.id} columns={[{ id: "a", header: "A", render: () => "x" }]} onAdd={() => {}} onRemove={() => {}} />,
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("DocumentRail", () => {
  it("menandai langkah aktif dan menautkan dokumen terkait", () => {
    render(
      <MemoryRouter>
        <DocumentRail
          steps={[
            { key: "draft", label: "Draf" },
            { key: "confirmed", label: "Terkonfirmasi" },
            { key: "paid", label: "Lunas" },
          ]}
          current="confirmed"
          related={[{ label: "SJ-0042", kind: "Surat jalan", to: "/deliveries/42", status: { label: "Dikirim", tone: "info" } }]}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText("Terkonfirmasi").closest("li")).toHaveAttribute("aria-current", "step");
    expect(screen.getByRole("link", { name: /SJ-0042/ })).toHaveAttribute("href", "/deliveries/42");
  });
});

describe("nilai & status", () => {
  it("Money gaya akuntansi: negatif dalam kurung, nol sebagai strip", () => {
    const { container } = render(
      <>
        <Money value={-1250000} accounting />
        <Money value={0} accounting />
        <Money value={5000} />
      </>,
    );
    expect(container.textContent).toContain("(Rp");
    expect(container.textContent).toContain("–");
  });

  it("StatusBadge memakai label Indonesia; DocNumber monospasi", () => {
    render(
      <>
        <StatusBadge status="cancelled" />
        <DocNumber value="SO-2026-00012" />
      </>,
    );
    expect(screen.getByText("Dibatalkan")).toBeInTheDocument();
    expect(screen.getByText("SO-2026-00012")).toHaveClass("font-mono");
  });
});

describe("Modal & ConfirmDialog (Radix)", () => {
  it("SearchSelect bisa dipakai DI DALAM Modal (popover tidak diblokir)", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <Modal open title="Form" onClose={() => {}}>
        <SearchSelect options={[{ value: 1, label: "Satu" }, { value: 2, label: "Dua" }]} value={null} onChange={onChange} />
      </Modal>,
    );
    const dialog = screen.getByRole("dialog", { name: "Form" });
    await user.click(within(dialog).getByRole("button", { name: "Pilih…" }));
    await user.click(await screen.findByRole("button", { name: "Dua" }));
    expect(onChange).toHaveBeenCalledWith(2);
  });

  it("Esc menutup Modal; ConfirmDialog: konfirmasi tidak memicu batal", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { unmount } = render(<Modal open title="X" onClose={onClose}>isi</Modal>);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
    unmount();

    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(<ConfirmDialog open title="Hapus?" message="Yakin?" confirmLabel="Ya, hapus" onConfirm={onConfirm} onCancel={onCancel} />);
    expect(screen.getByRole("alertdialog", { name: "Hapus?" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Ya, hapus" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });
});
