import { SectionCard } from "@/shared/components/ui";
import { DescriptionList } from "@/shared/components/patterns";

/** Tampilan baca "Informasi tambahan" customer/supplier (tak merender bila kosong). */
export function PartyAttributes({ attributes }: { attributes: Record<string, string> | undefined }) {
  const entries = Object.entries(attributes ?? {}).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) return null;
  return (
    <SectionCard title="Informasi tambahan">
      <DescriptionList items={entries.map(([k, v]) => ({ label: k, value: v }))} />
    </SectionCard>
  );
}
