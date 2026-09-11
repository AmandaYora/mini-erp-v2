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

async function rotate(): Promise<string | null> {
  if (!refreshing) {
    refreshing = (async () => {
      try {
        const refreshToken = tokenStore.refresh;
        if (!refreshToken) return null;
        const res = await axios.post(
          `${httpClient.defaults.baseURL}/api/v1/auth/refresh`,
          { refreshToken },
          { timeout: 15000 }
        );
        const pair = res.data?.data as { accessToken: string; refreshToken: string } | undefined;
        if (!pair?.accessToken) return null;
        tokenStore.set(pair.accessToken, pair.refreshToken);
        return pair.accessToken;
      } catch {
        return null;
      } finally {
        refreshing = null;
      }
    })();
  }
  return refreshing;
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
    const original = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined;
    const status = error.response?.status ?? 0;
    // Jangan refresh untuk endpoint auth sendiri (login/refresh gagal = kredensial salah).
    const url = original?.url ?? "";
    const isAuthCall = url.includes("/api/v1/auth/login") || url.includes("/api/v1/auth/refresh");
    if (status === 401 && original && !original._retried && !isAuthCall) {
      original._retried = true;
      const token = await rotate();
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
  return { status: 0, message: "Terjadi kesalahan tak terduga" };
}

/** GET yang mengembalikan data (tanpa meta). */
export async function apiGet<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const res = await httpClient.get<Envelope<T>>(url, { params });
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

export async function apiGetFull<T>(url: string, params?: Record<string, unknown>): Promise<MutateResult<T>> {
  const res = await httpClient.get<Envelope<T>>(url, { params });
  return toResult(res.data);
}

/** GET paginasi: {items, meta}. */
export async function apiPage<T>(url: string, params?: Record<string, unknown>): Promise<{ items: T[]; meta: PageMeta }> {
  const res = await httpClient.get<Envelope<T[]>>(url, { params });
  return {
    items: res.data.data ?? [],
    meta: res.data.meta ?? { page: 1, limit: 0, total: 0, totalPages: 0 },
  };
}

export async function apiPageFull<T>(
  url: string,
  params?: Record<string, unknown>,
): Promise<{ items: T[]; meta: PageMeta; message: string }> {
  const res = await httpClient.get<Envelope<T[]>>(url, { params });
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
