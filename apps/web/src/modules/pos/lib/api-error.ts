import { toApiError, type ApiError } from "@/shared/services/http-client";

/** Baca error dari http-client. Interceptor menolak dengan ApiError biasa
 * (bukan AxiosError), sedangkan toApiError() hanya mengenali AxiosError —
 * jadi bentuk ApiError dikembalikan apa adanya agar status + pesan server
 * (`message` envelope) tidak hilang. */
export function readApiError(err: unknown): ApiError {
  if (typeof err === "object" && err !== null) {
    const e = err as Partial<ApiError>;
    if (typeof e.message === "string" && typeof e.status === "number") {
      return { status: e.status, message: e.message, errors: e.errors };
    }
  }
  return toApiError(err);
}

/** Kegagalan yang pasti belum membuat apa pun di server (4xx kecuali 409
 * replay-konflik) — beda dengan jaringan putus / 5xx yang ambigu. */
export function isDefiniteRejection(err: unknown): boolean {
  const { status } = readApiError(err);
  return status >= 400 && status < 500;
}
