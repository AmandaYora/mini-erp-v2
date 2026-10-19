import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Money } from "@/shared/components/patterns";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn, AccountingRowKind } from "@/modules/finance/components/AccountingTable";

afterEach(cleanup);

interface Row {
  label: string;
  amount: number;
  kind: AccountingRowKind;
}

const columns: AccountingColumn<Row>[] = [
  { id: "label", header: "Keterangan", render: (r) => r.label },
  { id: "amount", header: "Jumlah", align: "right", render: (r) => <Money accounting value={r.amount} /> },
];

describe("AccountingTable", () => {
  it("memuat tabel ber-ARIA dengan header dan baris; negatif dalam kurung, nol berupa strip", () => {
    render(
      <AccountingTable<Row>
        caption="Laba rugi"
        columns={columns}
        rowKey={(r) => r.label}
        rowKind={(r) => r.kind}
        rows={[
          { label: "Pendapatan", amount: 1000, kind: "line" },
          { label: "HPP", amount: -400, kind: "line" },
          { label: "Lain-lain", amount: 0, kind: "line" },
          { label: "Laba kotor", amount: 600, kind: "total" },
        ]}
      />,
    );
    const table = screen.getByRole("table", { name: "Laba rugi" });
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual(["Keterangan", "Jumlah"]);
    expect(within(table).getAllByRole("row")).toHaveLength(5);
    expect(within(table).getByText(/^\(.*400\)$/)).toBeInTheDocument();
    expect(within(table).getByText("–")).toBeInTheDocument();
  });

  it("baris golongan hanya merender sel pertama", () => {
    render(
      <AccountingTable<Row>
        caption="Neraca"
        columns={columns}
        rowKey={(r) => r.label}
        rowKind={(r) => r.kind}
        rows={[{ label: "Aset", amount: 0, kind: "group" }]}
      />,
    );
    expect(screen.getAllByRole("cell")).toHaveLength(1);
  });

  it("rows null (Go) menampilkan keadaan kosong, galat menampilkan pesan API asli", () => {
    const { rerender } = render(<AccountingTable<Row> caption="x" columns={columns} rowKey={(r) => r.label} rows={null} emptyTitle="Kosong" />);
    expect(screen.getByText("Kosong")).toBeInTheDocument();
    rerender(<AccountingTable<Row> caption="x" columns={columns} rowKey={(r) => r.label} rows={[]} error="Server menolak" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Server menolak");
  });
});
