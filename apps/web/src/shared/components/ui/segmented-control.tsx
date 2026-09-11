export interface SegmentedOption {
  value: string;
  label: string;
  count?: number;
}

export interface SegmentedProps {
  options: SegmentedOption[];
  value: string;
  onChange: (value: string) => void;
}

export function Segmented({ options, value, onChange }: SegmentedProps) {
  return (
    <div className="flex flex-wrap gap-1 rounded-lg border border-hairline bg-surface-subtle p-1">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            className={`cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              active
                ? "bg-surface text-ink shadow-sm"
                : "text-muted hover:text-ink"
            }`}
          >
            {o.label}
            {o.count !== undefined && (
              <span className="bg-surface-sunken ml-1.5 rounded-full px-1.5 text-xs">
                {o.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
