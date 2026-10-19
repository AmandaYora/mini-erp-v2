import { useId } from "react";
import type { FormEvent, ReactNode } from "react";
import { Download } from "lucide-react";
import { Button, DateInput, FormField, SelectInput } from "@/shared/components/ui";
import { Toolbar } from "@/shared/components/patterns";
import { toast } from "@/shared/stores/toast.store";
import { downloadCsv } from "@/modules/finance/lib/csv";
import type { CsvCell } from "@/modules/finance/lib/csv";
import { MONTH_NAMES, yearOptions } from "@/modules/finance/lib/report-periods";

/**
 * Toolbar filter laporan (T5): isian periode/cabang + "Tampilkan"; `right` = aksi ekspor.
 * Berbentuk form sehingga Enter menerapkan filter.
 */
export function ReportToolbar({
  children,
  onApply,
  right,
  busy = false,
}: {
  children: ReactNode;
  onApply: () => void;
  right?: ReactNode;
  busy?: boolean;
}) {
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    onApply();
  }
  return (
    <form onSubmit={submit}>
      <Toolbar
        primary={
          <>
            {children}
            <Button type="submit" variant="secondary" loading={busy}>
              Tampilkan
            </Button>
          </>
        }
        right={right}
      />
    </form>
  );
}

export function ReportDateField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const id = useId();
  return (
    <FormField label={label} htmlFor={id}>
      <DateInput id={id} value={value} onChange={(e) => onChange(e.target.value)} />
    </FormField>
  );
}

/** Pilihan tahun + bulan (laporan bulanan). */
export function YearMonthFields({
  year,
  month,
  onYear,
  onMonth,
}: {
  year: number;
  month: number;
  onYear: (v: number) => void;
  onMonth: (v: number) => void;
}) {
  const yearId = useId();
  const monthId = useId();
  return (
    <>
      <FormField label="Tahun" htmlFor={yearId}>
        <SelectInput id={yearId} value={year} onChange={(e) => onYear(Number(e.target.value))}>
          {yearOptions().map((y) => (
            <option key={y} value={y}>
              {y}
            </option>
          ))}
        </SelectInput>
      </FormField>
      <FormField label="Bulan" htmlFor={monthId}>
        <SelectInput id={monthId} value={month} onChange={(e) => onMonth(Number(e.target.value))}>
          {MONTH_NAMES.map((label, i) => (
            <option key={label} value={i + 1}>
              {label}
            </option>
          ))}
        </SelectInput>
      </FormField>
    </>
  );
}

/** Ekspor CSV dari data yang sedang tampil (tanpa endpoint baru); nonaktif bila belum ada data. */
export function ExportCsvButton({
  filename,
  getRows,
}: {
  filename: string;
  /** null = belum ada data untuk diekspor. */
  getRows: () => CsvCell[][] | null;
}) {
  const rows = getRows();
  return (
    <Button
      variant="secondary"
      size="sm"
      disabled={rows === null}
      onClick={() => {
        if (!rows) return;
        downloadCsv(filename, rows);
        toast.success("Berkas CSV diunduh");
      }}
    >
      <Download aria-hidden /> Ekspor CSV
    </Button>
  );
}
