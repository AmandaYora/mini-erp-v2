import axios, { AxiosError, AxiosRequestConfig } from "axios";
import { loadingBus } from "@/shared/services/loading-bus";

// SATU-SATUNYA Axios instance (ditegakkan arch-check aturan D). Backend memakai
// envelope {success, message, data, meta?} — helper di bawah mengupasnya.

export interface Envelope<T> {
  success: boolean;
  message: string;
  data: T;
  meta?: PageMeta;
}

export interface PageMeta {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

export interface ApiError {
  status: number;
  message: string;
  errors?: { field: string; message: string }[];
}

export const httpClient = axios.create({
  // Empty = same origin. Proxied by Vite in dev, served by the same container in
  // production. Only set VITE_API_BASE_URL when web and api are deployed separately.
  baseURL: import.meta.env.VITE_API_BASE_URL || "",
  timeout: 30000,
  headers: { "Content-Type": "application/json" },
});

const ACCESS_KEY = "mini-erp.access-token";
const REFRESH_KEY = "mini-erp.refresh-token";

export const tokenStore = {
  get access() {
    return localStorage.getItem(ACCESS_KEY);
  },
  get refresh() {
    return localStorage.getItem(REFRESH_KEY);
  },
  set(access: string, refresh: string) {
    localStorage.setItem(ACCESS_KEY, access);
    localStorage.setItem(REFRESH_KEY, refresh);
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
  },
};

httpClient.interceptors.request.use((config) => {
  const token = tokenStore.access;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  loadingBus.begin();
  return config;
});

// Antrian refresh tunggal: 401 pertama memicu rotasi, request lain menunggu.
let refreshing: Promise<string | null> | null = null;

// Kode error backend saat dua tab memutar refresh token yang sama dalam
// jendela grace (F1-10, A-21): sesi tetap hidup, tab yang kalah memakai
// pasangan baru dari storage, bukan logout.
export const REFRESH_RACED_CODE = "refresh_raced";

function isRefreshRaced(error: unknown): boolean {
  if (!axios.isAxiosError(error)) return false;
  const data = error.response?.data as { code?: string } | undefined;
  return error.response?.status === 401 && data?.code === REFRESH_RACED_CODE;
}

/**
 * Refresh GAGAL KARENA TAK TERJANGKAU (jaringan putus, timeout, 5xx), bukan
 * karena token ditolak. Sesi belum tentu mati: token jangan dihapus dan
 * pengguna jangan ditendang ke login (draf/form tetap utuh); request asal
 * ditolak dengan galat jaringan biasa dan bisa dicoba lagi.
 */
export class RefreshUnreachableError extends Error {
  constructor() {
    super("Refresh sesi tidak terjangkau");
    this.name = "RefreshUnreachableError";
  }
}

/** Server MENJAWAB bahwa refresh token tidak berlaku (kedaluwarsa/dicabut). */
function isRefreshRejected(error: unknown): boolean {
  if (!axios.isAxiosError(error)) return false;
  const status = error.response?.status;
  return status === 400 || status === 401 || status === 403;
}

// Kunci silang-tab untuk rotasi (Web Locks API, F1-10). Tanpa ini dua tab
// yang 401 bersamaan memutar token yang sama: satu menang, satu dianggap
// replay dan SEMUA sesi user dicabut. Fallback: tanpa Web Locks (mis.
// konteks non-secure) rotasi berjalan tanpa kunci antar-tab.
const REFRESH_LOCK = "mini-erp-refresh";

type LockManager = { request<R>(name: string, fn: () => Promise<R>): Promise<R> };

function refreshLock(): LockManager | null {
  if (typeof navigator === "undefined") return null;
  const locks = (navigator as Navigator & { locks?: LockManager }).locks;
  return typeof locks?.request === "function" ? locks : null;
}

async function rotate(knownRefresh: string | null): Promise<string | null> {
  if (!refreshing) {
    const run = async (): Promise<string | null> => {
      try {
        const current = tokenStore.refresh;
        // Tab lain sudah memutar selagi kita antre kunci: pakai pasangan
        // barunya, jangan panggil /refresh dengan token basi.
        if (current && current !== knownRefresh) return tokenStore.access;
        if (!current) return null;
        try {
          const res = await axios.post(
            `${httpClient.defaults.baseURL}/api/v1/auth/refresh`,
            { refreshToken: current },
            { timeout: 15000 },
          );
          const pair = res.data?.data as { accessToken: string; refreshToken: string } | undefined;
          if (!pair?.accessToken) return null;
          tokenStore.set(pair.accessToken, pair.refreshToken);
          return pair.accessToken;
        } catch (e) {
          // Tab lain menang di server dalam jendela grace: ambil pasangan
          // terbaru yang ditulis tab pemenang ke storage.
          if (isRefreshRaced(e)) return tokenStore.access;
          // Hanya penolakan eksplisit server = sesi benar-benar habis.
          if (isRefreshRejected(e)) return null;
          throw new RefreshUnreachableError();
        }
      } finally {
        refreshing = null;
      }
    };
    const lock = refreshLock();
    refreshing = lock ? lock.request(REFRESH_LOCK, run) : run();
  }
  return refreshing;
}

// Diekspor untuk test (F1-10): mensimulasikan interleaving dua tab.
export { rotate as rotateForTest };

// Mengosongkan single-flight antar test.
export function resetRefreshForTest(): void {
  refreshing = null;
}

export type AuthExpiredHandler = () => void;
let onAuthExpired: AuthExpiredHandler | null = null;

/** Dipanggil sekali oleh auth store: refresh gagal total = paksa login ulang. */
export function setAuthExpiredHandler(fn: AuthExpiredHandler) {
  onAuthExpired = fn;
}

httpClient.interceptors.response.use(
  (res) => {
    loadingBus.end();
    return res;
  },
  async (error: AxiosError) => {
    // Pembatalan kita sendiri (ganti filter / pindah halaman / StrictMode dev menjalankan efek dua kali)
    // bukan galat jaringan: teruskan apa adanya agar isAbort() mengenalinya. Mengubahnya menjadi ApiError
    // ("Terjadi kesalahan jaringan") memunculkan toast palsu di setiap halaman yang menampilkan toast di catch.
    if (axios.isCancel(error)) {
      loadingBus.end();
      return Promise.reject(error);
    }
    const original = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined;
    const status = error.response?.status ?? 0;
    // Jangan refresh untuk endpoint auth sendiri (login/refresh gagal = kredensial salah).
    const url = original?.url ?? "";
    const isAuthCall = url.includes("/api/v1/auth/login") || url.includes("/api/v1/auth/refresh");
    if (status === 401 && original && !original._retried && !isAuthCall) {
      original._retried = true;
      // Token yang dikenal saat gagal: bila tab lain memutar selagi kita
      // antre, rotate memakai pasangan barunya tanpa /refresh ulang (F1-10).
      let token: string | null;
      try {
        token = await rotate(tokenStore.refresh);
      } catch (rotateErr) {
        if (rotateErr instanceof RefreshUnreachableError) {
          // Jaringan/server tak terjangkau: pertahankan token & halaman.
          loadingBus.end();
          return Promise.reject(toApiError(error));
        }
        throw rotateErr;
      }
      if (token) {
        // Retry dihitung sebagai request baru oleh interceptor request,
        // jadi respons 401 ini harus menutup hitungannya sendiri dulu.
        loadingBus.end();
        original.headers = { ...original.headers, Authorization: `Bearer ${token}` };
        return httpClient(original);
      }
      tokenStore.clear();
      onAuthExpired?.();
    }
    loadingBus.end();
    return Promise.reject(toApiError(error));
  }
);

export function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as { message?: string; errors?: ApiError["errors"] } | undefined;
    return {
      status: error.response?.status ?? 0,
      message: data?.message || "Terjadi kesalahan jaringan",
      errors: data?.errors,
    };
  }
  // Sudah berbentuk ApiError (interceptor menolak dengan ApiError): kembalikan
  // apa adanya, jangan timpa `message` server dengan pesan generik.
  if (
    typeof error === "object" &&
    error !== null &&
    typeof (error as ApiError).status === "number" &&
    typeof (error as ApiError).message === "string"
  ) {
    return error as ApiError;
  }
  return { status: 0, message: "Terjadi kesalahan tak terduga" };
}

/** Pesan per-field dari `errors[]` (tanpa yang kosong, ganda, atau sama dengan `message`). */
export function fieldErrorMessages(error: unknown): string[] {
  const e = toApiError(error);
  const seen = new Set<string>([e.message.trim()]);
  const out: string[] = [];
  for (const fe of e.errors ?? []) {
    const msg = (fe?.message ?? "").trim();
    if (msg === "" || seen.has(msg)) continue;
    seen.add(msg);
    out.push(msg);
  }
  return out;
}

/**
 * Teks galat lengkap untuk toast/notice: `message` server DIIKUTI pesan per-field.
 * Validasi backend menjawab `{message: "Validasi gagal", errors: [{field, message}]}`
 * — tanpa `errors[]` pengguna hanya melihat "Validasi gagal" tanpa alasannya.
 * Pakai ini di setiap handler tulis yang tidak memetakan `errors[]` ke input form.
 */
export function formatApiError(error: unknown): string {
  const e = toApiError(error);
  const details = fieldErrorMessages(e);
  return details.length > 0 ? `${e.message} — ${details.join("; ")}` : e.message;
}

/** True untuk request yang kita batalkan sendiri (ganti filter / pindah
 * halaman). Bukan kegagalan: tidak boleh muncul sebagai pesan error. */
export function isAbort(error: unknown): boolean {
  if (axios.isCancel(error)) return true;
  if (axios.isAxiosError(error)) return error.code === "ERR_CANCELED";
  return error instanceof DOMException && error.name === "AbortError";
}

/** GET yang mengembalikan data (tanpa meta). */
export async function apiGet<T>(
  url: string,
  params?: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<T> {
  const res = await httpClient.get<Envelope<T>>(url, { params, signal });
  return res.data.data;
}

/** GET yang mempertahankan envelope {data, message}. Pakai untuk aksi tulis
 * yang pesannya harus tampil apa adanya dari server (kontrak B1). */
export interface MutateResult<T> {
  data: T;
  message: string;
}

function toResult<T>(envelope: Envelope<T>): MutateResult<T> {
  return { data: envelope.data, message: envelope.message ?? "" };
}

/** Pilih pesan server bila ada, fallback ke teks klien bila kosong. */
export function pickServerMessage(serverMessage: string | undefined | null, fallback: string): string {
  const msg = (serverMessage ?? "").trim();
  return msg !== "" ? msg : fallback;
}

export async function apiGetFull<T>(
  url: string,
  params?: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<MutateResult<T>> {
  const res = await httpClient.get<Envelope<T>>(url, { params, signal });
  return toResult(res.data);
}

/** GET paginasi: {items, meta}. */
export async function apiPage<T>(
  url: string,
  params?: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<{ items: T[]; meta: PageMeta }> {
  const res = await httpClient.get<Envelope<T[]>>(url, { params, signal });
  return {
    items: res.data.data ?? [],
    meta: res.data.meta ?? { page: 1, limit: 0, total: 0, totalPages: 0 },
  };
}

export async function apiPageFull<T>(
  url: string,
  params?: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<{ items: T[]; meta: PageMeta; message: string }> {
  const res = await httpClient.get<Envelope<T[]>>(url, { params, signal });
  return {
    items: res.data.data ?? [],
    meta: res.data.meta ?? { page: 1, limit: 0, total: 0, totalPages: 0 },
    message: res.data.message ?? "",
  };
}

export async function apiPost<T>(url: string, body?: unknown): Promise<T> {
  const res = await httpClient.post<Envelope<T>>(url, body);
  return res.data.data;
}

export async function apiPostFull<T>(url: string, body?: unknown): Promise<MutateResult<T>> {
  const res = await httpClient.post<Envelope<T>>(url, body);
  return toResult(res.data);
}

export async function apiPut<T>(url: string, body?: unknown): Promise<T> {
  const res = await httpClient.put<Envelope<T>>(url, body);
  return res.data.data;
}

export async function apiPutFull<T>(url: string, body?: unknown): Promise<MutateResult<T>> {
  const res = await httpClient.put<Envelope<T>>(url, body);
  return toResult(res.data);
}

export async function apiPatch<T>(url: string, body?: unknown): Promise<T> {
  const res = await httpClient.patch<Envelope<T>>(url, body);
  return res.data.data;
}

export async function apiPatchFull<T>(url: string, body?: unknown): Promise<MutateResult<T>> {
  const res = await httpClient.patch<Envelope<T>>(url, body);
  return toResult(res.data);
}

export async function apiDelete<T>(url: string): Promise<T> {
  const res = await httpClient.delete<Envelope<T>>(url);
  return res.data.data;
}

export async function apiDeleteFull<T>(url: string): Promise<MutateResult<T>> {
  const res = await httpClient.delete<Envelope<T>>(url);
  return toResult(res.data);
}

/** POST multipart (unggah foto/bukti) — pesan error tetap dari envelope. */
export async function apiUpload<T>(url: string, form: FormData): Promise<T> {
  const res = await httpClient.post<Envelope<T>>(url, form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
  return res.data.data;
}

export async function apiUploadFull<T>(url: string, form: FormData): Promise<MutateResult<T>> {
  const res = await httpClient.post<Envelope<T>>(url, form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
  return toResult(res.data);
}
