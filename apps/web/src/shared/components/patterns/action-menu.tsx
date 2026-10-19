import type { ReactNode } from "react";
import { MoreHorizontal } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/components/primitives/dropdown-menu";
import { Button } from "@/shared/components/ui/button";
import { cn } from "@/shared/lib/cn";

export interface ActionMenuItem {
  key: string;
  label: string;
  icon?: ReactNode;
  onSelect: () => void;
  /** Aksi merusak: merah dan dipisahkan garis; letakkan paling bawah. */
  destructive?: boolean;
  disabled?: boolean;
}

/**
 * Menu "⋯" untuk aksi sekunder di header halaman atau baris tabel. Aksi utama (maksimal satu
 * tombol primary) tetap tombol terlihat; sisanya masuk sini (PLAN §2.2 prinsip 2).
 */
export function ActionMenu({
  items,
  label = "Aksi lainnya",
  align = "end",
  variant = "secondary",
}: {
  items: ActionMenuItem[];
  label?: string;
  align?: "start" | "end";
  variant?: "secondary" | "ghost";
}) {
  const normal = items.filter((i) => !i.destructive);
  const danger = items.filter((i) => i.destructive);
  if (items.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant={variant} size="icon" aria-label={label}>
          <MoreHorizontal aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="min-w-48">
        {normal.map((i) => (
          <DropdownMenuItem key={i.key} disabled={i.disabled} onSelect={i.onSelect}>
            {i.icon}
            {i.label}
          </DropdownMenuItem>
        ))}
        {normal.length > 0 && danger.length > 0 && <DropdownMenuSeparator />}
        {danger.map((i) => (
          <DropdownMenuItem
            key={i.key}
            disabled={i.disabled}
            onSelect={i.onSelect}
            className={cn("text-destructive focus:bg-destructive-soft focus:text-destructive")}
          >
            {i.icon}
            {i.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
