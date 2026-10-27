/**
 * Pilihan "Tampilkan rekening" di toolbar cetak (layar saja, tidak ikut tercetak).
 * Bawaan mati: nomor rekening hanya tercetak bila dicentang untuk cetakan itu, tidak
 * disimpan.
 */
export function ShowBankToggle({
  checked,
  onChange,
  className = "print-toolbar-option",
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
}) {
  return (
    <label className={className}>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>Tampilkan rekening</span>
    </label>
  );
}
