import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const chat = vi.fn();
const simulate = vi.fn();
vi.mock("@/modules/assistant/services/assistant.service", () => ({
  assistantService: {
    getChannel: vi.fn().mockResolvedValue({ connected: true, phone: "6281", qrDataUrl: "", lastError: "", updatedAt: "" }),
    listAuthorizations: vi.fn().mockResolvedValue([]),
    chat: (...a: unknown[]) => chat(...a),
    simulate: (...a: unknown[]) => simulate(...a),
  },
}));
vi.mock("@/modules/auth/hooks/use-accessible-branches", () => ({
  useAccessibleBranches: () => ({
    branches: [
      { id: 1, code: "JKT", name: "Cabang Pusat", status: "active" },
      { id: 2, code: "BDG", name: "Cabang Bandung", status: "active" },
    ],
    loading: false,
  }),
}));

import SetupPage from "@/modules/assistant/pages/SetupPage";

const answer = (branchName: string) => ({
  message: "Jawaban asisten",
  data: { runId: 1, branchId: 2, branchName, intent: "sales_summary", mode: "rule_based", answer: "Cabang: Cabang Bandung\nOmzet hari ini" },
});

beforeEach(() => {
  chat.mockReset();
  simulate.mockReset();
  chat.mockResolvedValue(answer("Cabang Bandung"));
  useAuthStore.setState({ permissions: ["assistant.view", "assistant.manage", "assistant.simulate"], ready: true });
});
afterEach(cleanup);

function renderConsole() {
  return render(
    <MemoryRouter initialEntries={["/assistant/setup?tab=konsol"]}>
      <SetupPage />
    </MemoryRouter>,
  );
}

describe("konsol uji asisten — pemilih cabang (AS-01)", () => {
  it("mengirim cabang terpilih (bukan branchId 0 tetap) dan 0 bila otomatis", async () => {
    renderConsole();
    const picker = (await screen.findAllByLabelText("Cabang"))[0] as HTMLSelectElement;
    // Opsi hanya cabang yang boleh diakses + "Otomatis".
    expect(Array.from(picker.options).map((o) => o.textContent)).toEqual([
      "Otomatis (dari pesan / semua cabang saya)",
      "JKT — Cabang Pusat",
      "BDG — Cabang Bandung",
    ]);

    fireEvent.change(screen.getByLabelText(/^Pesan/, { selector: "#assistant-chat-message" }), {
      target: { value: "omzet hari ini" },
    });
    fireEvent.change(picker, { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: /Kirim/ }));
    await waitFor(() => expect(chat).toHaveBeenCalledWith({ message: "omzet hari ini", branchId: 2 }));
    expect(await screen.findByText(/Cabang: Cabang Bandung/, { selector: "p.text-xs" })).toBeInTheDocument();

    fireEvent.change(picker, { target: { value: "0" } });
    fireEvent.change(screen.getByLabelText(/^Pesan/, { selector: "#assistant-chat-message" }), {
      target: { value: "omzet bandung" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Kirim/ }));
    await waitFor(() => expect(chat).toHaveBeenLastCalledWith({ message: "omzet bandung", branchId: 0 }));
  });
});
