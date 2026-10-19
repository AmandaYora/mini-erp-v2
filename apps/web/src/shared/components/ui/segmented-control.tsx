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
    <div className="inline-flex flex-wrap gap-0.5 rounded-lg bg-muted p-[3px]">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(o.value)}
            className={`cursor-pointer rounded-md px-3 py-1 text-[0.8125rem] font-medium transition-[background-color,color,box-shadow] duration-150 ${
              active
                ? "bg-card text-foreground shadow-[0_1px_2px_rgba(11,18,32,0.12)]"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {o.label}
            {o.count !== undefined && (
              <span
                className={`tabular ml-1.5 rounded-full px-1.5 text-[0.6875rem] ${
                  active ? "bg-accent text-primary" : "bg-card text-muted-foreground"
                }`}
              >
                {o.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
