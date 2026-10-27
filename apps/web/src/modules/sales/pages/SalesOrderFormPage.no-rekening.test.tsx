import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { draftKey, saveDraft } from "@/shared/lib/draft-storage";

// Rekening dipilih saat pembayaran dicatat, bukan di order (ADR-0026 revisi,
// analysis/005): form Order Jual tidak punya isian rekening walau perusahaan punya
// rekening, dan payload tidak membawa receiptAccountId.
const create = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: { create: (...a: unknown[]) => create(...a), get: vi.fn(), update: vi.fn() },
  };
});
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: () => <div data-testid="party-picker" />,
}));
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: { scanLookup: vi.fn(), detail: vi.fn() },
}));
vi.mock("@/modules/pos/components/ScanDialog", () => ({ ScanDialog: () => null }));
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: {
    listAddresses: vi.fn().mockResolvedValue([]),
    get: vi.fn(),
    create: vi.fn(),
    createAddress: vi.fn(),
  },
  memberTypeService: { list: vi.fn().mockResolvedValue([]) },
}));

const bankState = vi.fn();
vi.mock("@/modules/finance/hooks/use-bank-accounts", () => ({
  useBankAccounts: () => bankState(),
  invalidateBankAccounts: vi.fn(),
}));

vi.setConfig({ testTimeout: 30000 });

import SalesOrderFormPage from "@/modules/sales/pages/SalesOrderFormPage";

const line = {
  key: 1, productId: 5, productLabel: "BRG-1 — Barang Uji", variantId: 9, uom: "pcs", qty: 3,
  discountPct: 0, discountNominal: 0, catalogPrice: 50000, minPrice: 0, manualPrice: "",
  discountReason: "", note: "", taxable: true, pricingSource: "", uoms: ["pcs"],
  variants: [{ id: 9, code: "D", name: "Default" }],
};

function seedDraft(lines: unknown[]) {
  saveDraft(draftKey(1, "sales-order-new")!, {
    partyId: null, pickedCustomer: null, orderDate: "2026-10-18", paymentTerms: "cod", dueDate: "",
    notes: "", shipToAddressId: null, taxInvoiceNumber: "", taxInvoiceDate: "", lines,
  });
}

beforeEach(() => {
  create.mockReset().mockResolvedValue({ message: "Order dibuat", data: { id: 3 } });
  localStorage.clear();
  useAuthStore.setState({
    permissions: ["sales.create", "sales.view", "products.view"],
    ready: true,
    user: { id: 1, username: "u", fullName: "U" },
  } as never);
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <SalesOrderFormPage />
    </MemoryRouter>,
  );
}

const bca = {
  id: 1, accountId: 11, accountCode: "1011", accountName: "Bank BCA", bankName: "BCA", accountNumber: "123",
  accountHolder: "PT Uji", showOnDocuments: true, isDefault: true, sortOrder: 0, status: "active",
};
const mandiri = { ...bca, id: 2, accountId: 12, accountCode: "1012", bankName: "Mandiri", accountNumber: "456", isDefault: false };

describe("form order jual — tanpa rekening", () => {
  it("tidak ada isian rekening walau ada rekening perusahaan; payload tanpa receiptAccountId", async () => {
    bankState.mockReturnValue({ all: [bca, mandiri], active: [bca, mandiri], defaultAccountId: 11, required: true, loading: false });
    seedDraft([line]);
    renderPage();
    await screen.findByText(/BRG-1/);
    expect(screen.queryByLabelText(/Rekening/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0]).not.toHaveProperty("receiptAccountId");
  });
});
