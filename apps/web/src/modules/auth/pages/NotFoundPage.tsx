import { useLocation } from "react-router-dom";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";

/** Rute tak dikenal untuk pengguna yang sudah masuk (di dalam AppShell). */
export default function NotFoundPage() {
  const { pathname } = useLocation();
  return (
    <div className="py-10">
      <NotFoundState message={`Tidak ada halaman di "${pathname}". Periksa alamatnya atau kembali ke dashboard.`} />
    </div>
  );
}
