import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";

interface HintPos {
  left: number;
  top?: number;
  bottom?: number;
}

/**
 * FieldHint — tombol "?" kecil dengan popover keterangan. Diport dari aplikasi
 * lama. Dipakai di samping label untuk menjelaskan aturan isian tanpa
 * memakan tempat. Effect hanya berlangganan event (setState di callback).
 */
export function FieldHint({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<HintPos | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  function calcPos(): HintPos | null {
    if (!buttonRef.current || typeof window === "undefined") return null;
    const rect = buttonRef.current.getBoundingClientRect();
    return {
      bottom: window.innerHeight - rect.top + 6,
      left: rect.left + rect.width / 2,
    };
  }

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (buttonRef.current?.contains(e.target as Node)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  function handleToggle() {
    if (!open) setPos(calcPos());
    setOpen((v) => !v);
  }

  return (
    <span className="ml-[5px] inline-flex items-center align-middle">
      <button
        ref={buttonRef}
        type="button"
        onClick={handleToggle}
        aria-label="Keterangan"
        aria-expanded={open}
        className={[
          "inline-flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded-full border-0 p-0 text-[0.75rem] font-bold leading-none text-white transition-[background-color,opacity] duration-150",
          open ? "bg-brand opacity-100" : "bg-muted opacity-65",
        ].join(" ")}
      >
        ?
      </button>
      {open && pos && typeof document !== "undefined"
        ? createPortal(
            <div
              className="fixed z-[9999] max-w-[260px] -translate-x-1/2 rounded-md border border-hairline bg-surface px-3 py-2 text-[0.8rem] leading-normal text-heading shadow-[0_4px_16px_rgba(0,0,0,0.12)]"
              style={{ top: pos.top, bottom: pos.bottom, left: pos.left }}
            >
              {children}
            </div>,
            document.body,
          )
        : null}
    </span>
  );
}
