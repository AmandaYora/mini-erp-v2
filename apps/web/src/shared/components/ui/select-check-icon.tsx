import { SS_CHECK } from "./select-shared";

/** Ikon centang untuk item select yang aktif. File sendiri agar
 * select-shared.ts tetap murni konstanta (nol peringatan react-refresh). */
export function SelectCheckIcon() {
  return (
    <svg
      className={SS_CHECK}
      fill="none"
      height="16"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      width="16"
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}
