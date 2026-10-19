import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AuditDetail } from "@/modules/audit/components/AuditDetail";
import type { AuditRecord } from "@/modules/audit/types";

afterEach(cleanup);

function rec(over: Partial<AuditRecord>): AuditRecord {
  return {
    id: 1,
    action: "product.update",
    entity: "product",
    entityId: 5,
    branchId: 1,
    actorId: 2,
    actorType: "user",
    note: "",
    createdAt: "2026-10-09T10:00:00+07:00",
    ...over,
  };
}

describe("AuditDetail", () => {
  it("menampilkan hanya kunci yang berubah, gaya kode: baris - sebelum dan + sesudah dengan jenisnya", () => {
    render(
      <AuditDetail
        record={rec({
          before: { name: "A", price: 100, old: 1 },
          after: { name: "B", price: 100, fresh: true },
        })}
      />,
    );
    const rows = within(screen.getByRole("region", { name: "Perubahan" })).getAllByRole("listitem");
    expect(rows).toHaveLength(3);
    const byKey = Object.fromEntries(rows.map((r) => [r.querySelector("span.font-mono")?.textContent, r]));

    // diubah: kedua baris (sebelum & sesudah)
    expect(byKey.name).toHaveTextContent("Diubah");
    expect(byKey.name).toHaveTextContent("Sebelum: A");
    expect(byKey.name).toHaveTextContent("Sesudah: B");
    // ditambah: hanya baris sesudah
    expect(byKey.fresh).toHaveTextContent("Ditambah");
    expect(byKey.fresh).toHaveTextContent("Sesudah: true");
    expect(byKey.fresh).not.toHaveTextContent("Sebelum:");
    // dihapus: hanya baris sebelum
    expect(byKey.old).toHaveTextContent("Dihapus");
    expect(byKey.old).toHaveTextContent("Sebelum: 1");
    expect(byKey.old).not.toHaveTextContent("Sesudah:");
    expect(screen.queryByText("price")).not.toBeInTheDocument();
  });

  it("before kosong (entri baru) = semua ditambah; metadata tampil sebagai JSON", () => {
    render(<AuditDetail record={rec({ before: null, after: { a: 1 }, metadata: { ip: "10.0.0.1" } })} />);
    expect(screen.getByText("Ditambah")).toBeInTheDocument();
    expect(screen.getByText(/"ip": "10.0.0.1"/)).toBeInTheDocument();
  });

  it("sebelum dan sesudah identik memberi keterangan", () => {
    render(<AuditDetail record={rec({ before: { a: 1 }, after: { a: 1 } })} />);
    expect(screen.getByText(/tidak ada perbedaan/)).toBeInTheDocument();
  });

  it("menampilkan konteks entri: aktor, entitas, cabang", () => {
    render(<AuditDetail record={rec({ actorName: "Siti", branchName: "Cabang Dua", before: { a: 1 }, after: { a: 2 } })} />);
    expect(screen.getByText("Siti")).toBeInTheDocument();
    expect(screen.getByText("Produk #5")).toBeInTheDocument();
    expect(screen.getByText("Cabang Dua")).toBeInTheDocument();
  });
});
