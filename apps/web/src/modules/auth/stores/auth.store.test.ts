import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { tokenStore } from "@/shared/services/http-client";

const me = vi.fn();
const switchRole = vi.fn();
vi.mock("@/modules/auth/services/auth.service", () => ({
  authService: {
    me: (...a: unknown[]) => me(...a),
    login: vi.fn(),
    logout: vi.fn(),
    switchBranch: vi.fn(),
    switchRole: (...a: unknown[]) => switchRole(...a),
  },
}));

import { useAuthStore } from "@/modules/auth/stores/auth.store";

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  tokenStore.set("A", "R");
  useAuthStore.setState({ ready: false, user: null, branch: null, permissions: [], company: null, bootError: null });
});
afterEach(() => localStorage.clear());

describe("loadMe", () => {
  it("sukses: sesi terisi dan bootError bersih", async () => {
    me.mockResolvedValue({ user: { id: 1 }, branch: { id: 2 }, permissions: ["x"], company: null });
    await useAuthStore.getState().loadMe();
    const s = useAuthStore.getState();
    expect(s.user).toMatchObject({ id: 1 });
    expect(s.bootError).toBeNull();
    expect(s.ready).toBe(true);
  });

  it("jaringan putus: token dipertahankan, bootError network, bukan logout", async () => {
    me.mockRejectedValue({ status: 0, message: "Terjadi kesalahan jaringan" });
    await useAuthStore.getState().loadMe();
    const s = useAuthStore.getState();
    expect(s.bootError).toBe("network");
    expect(s.ready).toBe(true);
    expect(tokenStore.access).toBe("A");
    expect(tokenStore.refresh).toBe("R");
  });

  it("5xx diperlakukan sama dengan jaringan putus", async () => {
    me.mockRejectedValue({ status: 503, message: "x" });
    await useAuthStore.getState().loadMe();
    expect(useAuthStore.getState().bootError).toBe("network");
    expect(tokenStore.access).toBe("A");
  });

  it("401 ditolak server: token dibuang", async () => {
    me.mockRejectedValue({ status: 401, message: "tidak berwenang" });
    await useAuthStore.getState().loadMe();
    expect(useAuthStore.getState().bootError).toBeNull();
    expect(tokenStore.access).toBeNull();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("coba lagi setelah gagal jaringan memulihkan sesi", async () => {
    me.mockRejectedValueOnce({ status: 0, message: "x" });
    await useAuthStore.getState().loadMe();
    me.mockResolvedValueOnce({ user: { id: 1 }, branch: { id: 2 }, permissions: [], company: null });
    await useAuthStore.getState().loadMe();
    expect(useAuthStore.getState().bootError).toBeNull();
    expect(useAuthStore.getState().user).toMatchObject({ id: 1 });
  });
});

describe("role aktif", () => {
  it("loadMe menyimpan activeRole dan availableRoles", async () => {
    me.mockResolvedValue({
      user: { id: 1 },
      branch: { id: 2 },
      permissions: [],
      company: null,
      activeRole: { id: 1, code: "admin", name: "Admin" },
      availableRoles: [
        { id: 1, code: "admin", name: "Admin" },
        { id: 2, code: "kasir", name: "Kasir" },
      ],
    });
    await useAuthStore.getState().loadMe();
    const s = useAuthStore.getState();
    expect(s.activeRole?.code).toBe("admin");
    expect(s.availableRoles).toHaveLength(2);
  });

  it("respons lama tanpa role tidak merusak state", async () => {
    me.mockResolvedValue({ user: { id: 1 }, branch: { id: 2 }, permissions: [], company: null });
    await useAuthStore.getState().loadMe();
    expect(useAuthStore.getState().activeRole).toBeNull();
    expect(useAuthStore.getState().availableRoles).toEqual([]);
  });

  it("switchRole: token baru disimpan lalu izin dimuat ulang", async () => {
    switchRole.mockResolvedValue({ accessToken: "B" });
    me.mockResolvedValue({ user: { id: 1 }, branch: { id: 2 }, permissions: ["stock.adjust"], company: null });
    await useAuthStore.getState().switchRole(2);
    expect(switchRole).toHaveBeenCalledWith(2);
    expect(tokenStore.access).toBe("B");
    expect(tokenStore.refresh).toBe("R");
    expect(useAuthStore.getState().can("stock.adjust")).toBe(true);
  });
});
