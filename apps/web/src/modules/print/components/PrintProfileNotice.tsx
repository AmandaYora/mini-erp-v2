interface Props {
  /** Profil kertas tersimpan rusak — default dipakai. */
  corrupt: boolean;
  /** Profil perusahaan gagal dimuat (mis. tanpa izin) — kop/rekening kosong. */
  warning: string | null;
}

/** Peringatan non-blokir di atas dokumen cetak; tidak ikut tercetak. */
export function PrintProfileNotice({ corrupt, warning }: Props) {
  if (!corrupt && warning === null) return null;
  return (
    <>
      {corrupt ? (
        <div className="no-print print-warning">
          Profil kertas tersimpan rusak — memakai default 241,3 × 139,7 mm. Perbaiki di kalibrasi kertas.
        </div>
      ) : null}
      {warning !== null ? (
        <div className="no-print print-warning" role="alert">
          {warning}
        </div>
      ) : null}
    </>
  );
}
