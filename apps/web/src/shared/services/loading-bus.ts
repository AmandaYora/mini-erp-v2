// Bus penghitung request in-flight untuk GlobalLoader.
//
// Hidup di luar React (ality axios interceptors di http-client menambah /
// mengurangi), dibaca React via useSyncExternalStore (useGlobalLoading).
// Satu-satunya state bersama: jumlah request yang belum selesai.

type Listener = () => void;

let inFlight = 0;
const listeners = new Set<Listener>();

function emit() {
  for (const fn of listeners) fn();
}

export const loadingBus = {
  begin() {
    inFlight += 1;
    emit();
  },
  end() {
    inFlight = Math.max(0, inFlight - 1);
    emit();
  },
  subscribe(fn: Listener): () => void {
    listeners.add(fn);
    return () => {
      listeners.delete(fn);
    };
  },
  isLoading(): boolean {
    return inFlight > 0;
  },
  /** Diekspos untuk test saja — mengembalikan penghitung ke nol. */
  resetForTest() {
    inFlight = 0;
    emit();
  },
};
