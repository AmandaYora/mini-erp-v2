import { useCallback, useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Volume2, VolumeX, X } from "lucide-react";
import { Button, Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import { cameraFallbackMessage, cameraUnavailableReason, pickRearCamera } from "@/modules/pos/lib/camera";
import { isScanSoundMuted, playScanSound, setScanSoundMuted } from "@/modules/pos/lib/scan-sound";
import {
  EMPTY_SCAN_GATE,
  chipFor,
  gateScan,
  pushChip,
  type ScanChip,
  type ScanGate,
  type ScanOutcome,
} from "@/modules/pos/lib/scan-session";

// Pindai kamera beruntun via @zxing/browser (dynamic import agar bundle awal kecil).
// Dialog tetap terbuka: tiap kode terbaca -> kilat hijau + getar + chip + barang masuk
// keranjang; kode sama dalam ±1,5 dtk diabaikan (lib/scan-session). Kamera butuh HTTPS + izin;
// bila tak tersedia / ditolak, input kode manual selalu tersedia sebagai cadangan.

interface ScannerControls {
  stop: () => void;
}
interface DecodeResult {
  getText: () => string;
}

const CHIP_TONE = {
  success: "bg-success-soft text-success",
  warning: "bg-warning-soft text-warning",
  danger: "bg-destructive-soft text-destructive",
} as const;

// Galat per-bingkai yang normal (belum ada kode / bacaan sebagian) — bukan kegagalan kamera.
const BENIGN_FRAME_ERROR = /NotFound|Checksum|Format/i;

function ScanBody({
  paused,
  onClose,
  onScan,
}: {
  paused: boolean;
  onClose: () => void;
  onScan: (code: string) => Promise<ScanOutcome>;
}) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const controlsRef = useRef<ScannerControls | null>(null);
  const onScanRef = useRef(onScan);
  const pausedRef = useRef(paused);
  const gateRef = useRef<ScanGate>(EMPTY_SCAN_GATE);
  const queueRef = useRef<Promise<void>>(Promise.resolve());
  const chipId = useRef(0);
  const flashTimer = useRef<number | null>(null);

  useEffect(() => {
    onScanRef.current = onScan;
    pausedRef.current = paused;
  });

  const [devices, setDevices] = useState<{ deviceId: string; label: string }[]>([]);
  // "" = belum memilih; "default" = tanpa daftar perangkat (pakai kamera belakang bawaan).
  const [deviceId, setDeviceId] = useState("");
  const [status, setStatus] = useState<"loading" | "scanning" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const [manual, setManual] = useState("");
  const [chips, setChips] = useState<ScanChip[]>([]);
  const [count, setCount] = useState(0);
  const [flash, setFlash] = useState<ScanChip["tone"] | null>(null);
  const [muted, setMuted] = useState(isScanSoundMuted);

  useEffect(
    () => () => {
      if (flashTimer.current !== null) window.clearTimeout(flashTimer.current);
    },
    [],
  );

  /** Proses satu kode secara berurutan (antrean) lalu beri umpan balik. */
  const process = useCallback((code: string) => {
    queueRef.current = queueRef.current.then(async () => {
      const outcome = await onScanRef.current(code);
      const chip = chipFor(outcome);
      setChips((list) => pushChip(list, { id: ++chipId.current, ...chip }));
      // Bunyi untuk kode yang dikenal (masuk, atau perlu pilih varian) — bukan galat.
      if (chip.tone !== "danger") playScanSound();
      if (outcome.kind === "added") {
        setCount((n) => n + 1);
        if (typeof navigator !== "undefined" && typeof navigator.vibrate === "function") navigator.vibrate(40);
      }
      setFlash(chip.tone);
      if (flashTimer.current !== null) window.clearTimeout(flashTimer.current);
      flashTimer.current = window.setTimeout(() => setFlash(null), 450);
    });
  }, []);

  const handleFrame = useCallback(
    (raw: string) => {
      if (pausedRef.current) return;
      const r = gateScan(gateRef.current, raw, Date.now());
      gateRef.current = r.gate;
      if (r.accept) process(r.gate.code ?? raw.trim());
    },
    [process],
  );

  useEffect(() => {
    let alive = true;
    let controls: ScannerControls | null = null;

    void (async () => {
      const reason = cameraUnavailableReason();
      if (reason) {
        setError(cameraFallbackMessage(reason));
        setStatus("error");
        return;
      }
      try {
        const { BrowserMultiFormatReader } = await import("@zxing/browser");
        if (!alive) return;
        if (deviceId === "") {
          const list: { deviceId: string; label: string }[] = await BrowserMultiFormatReader.listVideoInputDevices();
          if (!alive) return;
          setDevices(list.map((d) => ({ deviceId: d.deviceId, label: d.label })));
          setDeviceId(pickRearCamera(list)?.deviceId || "default");
          return; // efek berjalan lagi dengan kamera terpilih
        }
        const video = videoRef.current;
        if (!video) {
          setError("Pratinjau kamera tidak tersedia — gunakan input manual di bawah.");
          setStatus("error");
          return;
        }
        const reader = new BrowserMultiFormatReader();
        controls = await reader.decodeFromVideoDevice(
          deviceId === "default" ? undefined : deviceId,
          video,
          (result: DecodeResult | undefined, err: unknown) => {
            if (!alive) return;
            if (result) {
              handleFrame(result.getText());
            } else if (err instanceof Error && !BENIGN_FRAME_ERROR.test(err.name)) {
              setError(`Kamera gagal membaca (${err.name}) — gunakan input manual di bawah.`);
              setStatus("error");
            }
          },
        );
        controlsRef.current = controls;
        if (!alive) {
          controls.stop();
          controlsRef.current = null;
          return;
        }
        setError(null);
        setStatus("scanning");
      } catch (err) {
        if (!alive) return;
        const msg = err instanceof Error ? `${err.name} ${err.message}` : String(err);
        if (/NotAllowed|Permission|Denied/i.test(msg)) {
          setError("Akses kamera ditolak — izinkan di pengaturan browser atau gunakan input manual di bawah.");
        } else if (/NotFound|DevicesNotFound|Overconstrained/i.test(msg)) {
          setError("Kamera tidak ditemukan — gunakan input manual di bawah.");
        } else {
          setError("Kamera tidak dapat dibuka — gunakan input manual di bawah.");
        }
        setStatus("error");
      }
    })();

    return () => {
      alive = false;
      controls?.stop();
      controlsRef.current?.stop();
      controlsRef.current = null;
    };
  }, [deviceId, handleFrame]);

  function handleManual(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const code = manual.trim();
    if (code === "") return;
    setManual("");
    process(code); // input manual disengaja: tidak ikut peredam kode sama
  }

  return (
    <>
      <SheetHeader className="flex-row items-center justify-between gap-3 p-4 text-left">
        <div className="min-w-0">
          <SheetTitle className="font-display text-xl text-sidebar-strong">Pindai barang</SheetTitle>
          <SheetDescription className="text-sm text-sidebar-foreground">
            Arahkan kamera ke QR label (atau barcode pabrik) barang. Dialog tetap terbuka untuk barang berikutnya.
          </SheetDescription>
        </div>
        <button
          type="button"
          aria-label={muted ? "Nyalakan bunyi pindai" : "Bisukan bunyi pindai"}
          aria-pressed={muted}
          onClick={() => {
            setScanSoundMuted(!muted);
            setMuted(!muted);
          }}
          className="flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-sidebar-strong select-none active:bg-sidebar-active"
        >
          {muted ? <VolumeX aria-hidden className="size-6" /> : <Volume2 aria-hidden className="size-6" />}
        </button>
        <button
          type="button"
          aria-label="Tutup"
          onClick={onClose}
          className="flex size-12 flex-none touch-manipulation items-center justify-center rounded-lg text-sidebar-strong select-none active:bg-sidebar-active"
        >
          <X aria-hidden className="size-6" />
        </button>
      </SheetHeader>

      <div className="relative mx-4 min-h-0 flex-1 overflow-hidden rounded-2xl bg-foreground">
        <video ref={videoRef} className="size-full object-cover" muted playsInline />
        {status === "loading" && (
          <p role="status" className="absolute inset-0 flex items-center justify-center text-base text-background/80">
            Membuka kamera…
          </p>
        )}
        {status === "scanning" && (
          <div
            aria-hidden
            className={cn(
              "pointer-events-none absolute inset-6 rounded-2xl border-4 transition-colors duration-150",
              flash === "success" && "border-success bg-success/25",
              flash === "warning" && "border-warning bg-warning/20",
              flash === "danger" && "border-destructive bg-destructive/20",
              flash === null && "border-background/70",
            )}
          />
        )}
        {devices.length >= 2 && (
          <div role="group" aria-label="Kamera" className="absolute right-3 bottom-3 left-3 flex gap-2 overflow-x-auto">
            {devices.map((d, i) => (
              <button
                key={d.deviceId}
                type="button"
                aria-pressed={deviceId === d.deviceId}
                onClick={() => setDeviceId(d.deviceId)}
                className={cn(
                  "h-12 flex-none touch-manipulation rounded-full px-4 text-sm font-medium whitespace-nowrap select-none",
                  deviceId === d.deviceId ? "bg-background text-foreground" : "bg-foreground/60 text-background",
                )}
              >
                {d.label || `Kamera ${i + 1}`}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-3 p-4">
        {error && <Notice tone="warning" title={error} />}
        <div aria-live="polite" className="flex min-h-11 flex-wrap gap-2">
          {chips.map((c) => (
            <span key={c.id} className={cn("max-w-full truncate rounded-full px-4 py-2 text-sm font-semibold", CHIP_TONE[c.tone])}>
              {c.text}
            </span>
          ))}
        </div>
        <form onSubmit={handleManual} className="flex gap-2">
          <input
            aria-label="Kode manual"
            value={manual}
            onChange={(e) => setManual(e.target.value)}
            placeholder="Ketik / tempel kode (scanner USB)…"
            autoComplete="off"
            className="h-12 min-w-0 flex-1 rounded-lg border border-sidebar-border bg-sidebar-accent px-4 text-base text-sidebar-strong placeholder:text-sidebar-muted focus:border-sidebar-ring focus:ring-2 focus:ring-sidebar-ring/40 focus:outline-none"
          />
          <Button type="submit" variant="secondary" className="h-12 px-5 text-base" disabled={manual.trim() === ""}>
            Tambah
          </Button>
        </form>
      </div>

      <SheetFooter className="flex-none p-4 pt-0">
        <Button className="h-14 w-full text-lg font-semibold" onClick={onClose}>
          Selesai ({count} barang)
        </Button>
      </SheetFooter>
    </>
  );
}

/** Dialog pindai kamera layar penuh, pindai beruntun. `paused` menahan bacaan (mis. saat memilih varian). */
export function ScanDialog({
  open,
  paused,
  onClose,
  onScan,
}: {
  open: boolean;
  paused: boolean;
  onClose: () => void;
  onScan: (code: string) => Promise<ScanOutcome>;
}) {
  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent
        side="bottom"
        showCloseButton={false}
        className="inset-0 flex h-dvh max-h-none w-full gap-0 rounded-none border-0 bg-sidebar p-0 text-sidebar-strong"
      >
        {open && <ScanBody paused={paused} onClose={onClose} onScan={onScan} />}
      </SheetContent>
    </Sheet>
  );
}
