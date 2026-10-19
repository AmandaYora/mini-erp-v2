import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PosCustomerPanel } from "@/modules/pos/components/PosCustomerPanel";
import type { PosCustomerDetail, PricingQuote } from "@/modules/pos/types";

afterEach(cleanup);

const gold: PosCustomerDetail = {
  id: 9, code: "CUS-009", name: "Siti Aminah", phone: "0812345678",
  member: { id: 4, code: "GOLD" },
  addresses: [],
};

const quote = (over: Partial<PricingQuote> = {}): PricingQuote => ({
  memberType: "GOLD", memberInactive: false, lines: [], ...over,
});

function renderPanel(over: Partial<React.ComponentProps<typeof PosCustomerPanel>> = {}) {
  const onChange = vi.fn();
  render(
    <PosCustomerPanel
      customer={null}
      memberName={null}
      quote={null}
      quoteStatus="idle"
      quoteError={null}
      savings={0}
      locked={false}
      onChange={onChange}
      {...over}
    />,
  );
  return { onChange };
}

describe("PosCustomerPanel", () => {
  it("tanpa pelanggan: Umum / Walk-in dengan petunjuk harga katalog; Ganti membuka pemilih", () => {
    const { onChange } = renderPanel();
    expect(screen.getByText("Umum / Walk-in")).toBeInTheDocument();
    expect(screen.getByText(/Harga katalog berlaku/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ganti pelanggan" }));
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("pelanggan member: nama, telepon, badge tipe member, dan total hemat", () => {
    renderPanel({ customer: gold, memberName: "Gold", quote: quote(), savings: 12500 });
    expect(screen.getByText("Siti Aminah")).toBeInTheDocument();
    expect(screen.getByText("0812345678")).toBeInTheDocument();
    expect(screen.getByText("Gold")).toBeInTheDocument();
    expect(screen.getByText(/Hemat .*12\.500.* dengan harga member/)).toBeInTheDocument();
  });

  it("member tanpa nama tipe memakai kode; tanpa telepon menampilkan kode pelanggan", () => {
    renderPanel({ customer: { ...gold, phone: "  " }, memberName: null, quote: quote(), savings: 0 });
    expect(screen.getByText("GOLD")).toBeInTheDocument();
    expect(screen.getByText("CUS-009")).toBeInTheDocument();
    expect(screen.getByText(/sama dengan harga katalog/)).toBeInTheDocument();
  });

  it("pelanggan non-member ditandai Non-member tanpa baris hemat", () => {
    renderPanel({ customer: { ...gold, member: null }, quote: quote(), savings: 0 });
    expect(screen.getByText("Non-member")).toBeInTheDocument();
    expect(screen.queryByText(/Hemat/)).not.toBeInTheDocument();
    expect(screen.queryByText(/sama dengan harga katalog/)).not.toBeInTheDocument();
  });

  it("member nonaktif: peringatan tampil dan hemat disembunyikan", () => {
    renderPanel({ customer: gold, memberName: "Gold", quote: quote({ memberInactive: true }), savings: 5000 });
    expect(screen.getByText("Member nonaktif")).toBeInTheDocument();
    expect(screen.queryByText(/Hemat/)).not.toBeInTheDocument();
  });

  it("status harga: memuat dan galat", () => {
    renderPanel({ customer: gold, quoteStatus: "loading" });
    expect(screen.getByText("Menghitung harga member…")).toBeInTheDocument();
    cleanup();
    renderPanel({ customer: gold, quoteStatus: "error", quoteError: "Harga member gagal dihitung" });
    expect(screen.getByText("Harga member gagal dihitung")).toBeInTheDocument();
  });

  it("terkunci (keranjang sedang dibayar): tombol Ganti nonaktif", () => {
    const { onChange } = renderPanel({ locked: true });
    const btn = screen.getByRole("button", { name: "Ganti pelanggan" });
    expect(btn).toBeDisabled();
    fireEvent.click(btn);
    expect(onChange).not.toHaveBeenCalled();
  });
});
