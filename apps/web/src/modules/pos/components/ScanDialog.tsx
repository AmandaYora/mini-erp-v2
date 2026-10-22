import { useCallback, useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { SwitchCamera, Volume2, VolumeX, X } from "lucide-react";
import { Button, Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { cn } from "@/shared/lib/cn";
import {
  cameraActiveMessage,
  cameraAttempts,
  cameraFallbackMessage,
  cameraUnavailableReason,
  classifyCameras,
  loadCameraSide,
  nextCamera,
  saveCameraSide,
  sideFromFacingMode,
  switchCameraLabel,
  type CameraInfo,
  type CameraRequest,
  type CameraSide,
} from "@/modules/pos/lib/camera";
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
// Kamera dibuka dulu (facingMode), baru daftar perangkat dibaca: sebelum izin browser
// hanya memberi satu perangkat tanpa label. Tombol ganti kamera muncul bila ada dua sisi
// (atau >=2 webcam); tiap ganti menghentikan stream lama sebelum membuka yang baru.

interface ScannerControls {
  // zxing: async bila track mendukung senter (mematikan senter setelah berhenti).
  stop: () => void | Promise<void>;
  streamVideoSettingsGet?: (trackFilter: (track: MediaStreamTrack) => MediaStreamTrack[]) => MediaTrackSettings;
}
interface DecodeResult {
  getText: () => string;
}
type DecodeCallback = (result: DecodeResult | undefined, err: unknown) => void;
interface ScanReader {
  decodeFromConstraints: (
    constraints: MediaStreamConstraints,
    video: HTMLVideoElement,
    callback: DecodeCallback,
  ) => Promise<ScannerControls>;
}

const CHIP_TONE = {
  success: "bg-success-soft text-success",
  warning: "bg-warning-soft text-warning",
  danger: "bg-destructive-soft text-destructive",
} as const;

// Galat per-bingkai yang normal (belum ada kode / bacaan sebagian) — bukan kegagalan kamera.
const BENIGN_FRAME_ERROR = /NotFound|Checksum|Format/i;
// Stream baru segera setelah stop kadang gagal/hitam di iOS Safari: ulangi sekali setelah jeda.
const SWITCH_RETRY_MS = 400;

// Runtime zxing 0.2.1 memanggil videoTracks.find(filter); d.ts-nya menulis filter mengembalikan array.
const FIRST_TRACK = (() => true) as unknown as (track: MediaStreamTrack) => MediaStreamTrack[];

function errorText(err: unknown): string {
  return err instanceof Error ? `${err.name} ${err.message}` : String(err);
}

async function stopCamera(controls: ScannerControls | null): Promise<void> {
  try {
    await controls?.stop();
  } catch {
    /* senter gagal dimatikan: track sudah berhenti */
  }
}

/** Coba tiap constraint berurutan (berikutnya bila Overconstrained/NotFound); saat ganti
 * kamera, galat lain diulang sekali setelah jeda. */
async function openCamera(
  reader: ScanReader,
  video: HTMLVideoElement,
  request: CameraRequest,
  callback: DecodeCallback,
  isAlive: () => boolean,
): Promise<ScannerControls> {
  let lastErr: unknown = new Error("NotFoundError");
  for (const constraints of cameraAttempts(request)) {
    for (let attempt = 0; ; attempt++) {
      try {
        return await reader.decodeFromConstraints(constraints, video, callback);
      } catch (err) {
        lastErr = err;
        const msg = errorText(err);
        if (!isAlive() || /NotAllowed|Permission|Denied/i.test(msg)) throw err;
        if (/Overconstrained|NotFound/i.test(msg)) break;
        if (request.kind === "initial" || attempt > 0) throw err;
        await new Promise((resolve) => window.setTimeout(resolve, SWITCH_RETRY_MS));
        if (!isAlive()) throw err;
      }
    }
  }
  throw lastErr;
}

function readTrackSettings(controls: ScannerControls): MediaTrackSettings {
  try {
    return controls.streamVideoSettingsGet?.(FIRST_TRACK) ?? {};
  } catch {
    return {};
  }
}

async function listVideoInputs(): Promise<MediaDeviceInfo[]> {
  try {
    return (await navigator.mediaDevices.enumerateDevices?.()) ?? [];
  } catch {
    return [];
  }
}

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
  const readerRef = useRef<ScanReader | null>(null);
  // Buka/tutup kamera berurutan: stream baru tidak dibuka sebelum yang lama berhenti.
  const cameraChain = useRef<Promise<void>>(Promise.resolve());
  const settingsRef = useRef<MediaTrackSettings>({});
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

  const [request, setRequest] = useState<CameraRequest>(() => ({ kind: "initial", side: loadCameraSide() }));
  const [cameras, setCameras] = useState<CameraInfo[]>([]);
  const [active, setActive] = useState<{ deviceId: string | null; side: CameraSide | null }>({
    deviceId: null,
    side: null,
  });
  const [announce, setAnnounce] = useState("");
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

    const run = cameraChain.current.then(async () => {
      if (!alive) return;
      const reason = cameraUnavailableReason();
      if (reason) {
        setError(cameraFallbackMessage(reason));
        setStatus("error");
        return;
      }
      try {
        const { BrowserMultiFormatReader } = await import("@zxing/browser");
        if (!alive) return;
        const video = videoRef.current;
        if (!video) {
          setError("Pratinjau kamera tidak tersedia — gunakan input manual di bawah.");
          setStatus("error");
          return;
        }
        readerRef.current ??= new BrowserMultiFormatReader();
        const opened = await openCamera(
          readerRef.current,
          video,
          request,
          (result, err) => {
            if (!alive) return;
            if (result) {
              handleFrame(result.getText());
            } else if (err instanceof Error && !BENIGN_FRAME_ERROR.test(err.name)) {
              setError(`Kamera gagal membaca (${err.name}) — gunakan input manual di bawah.`);
              setStatus("error");
            }
          },
          () => alive,
        );
        if (!alive) {
          await stopCamera(opened);
          return;
        }
        controls = opened;
        setError(null);
        setStatus("scanning");

        // Setelah izin: label & deviceId asli tersedia; sisi kamera aktif dari settings track.
        const settings = readTrackSettings(opened);
        settingsRef.current = settings;
        const list = classifyCameras(await listVideoInputs(), settings);
        if (!alive) return;
        const activeId = settings.deviceId ?? null;
        const side = list.find((c) => c.deviceId === activeId)?.side ?? sideFromFacingMode(settings.facingMode);
        setCameras(list);
        setActive({ deviceId: activeId, side });
        if (request.kind !== "initial") {
          if (side) saveCameraSide(side);
          setAnnounce(cameraActiveMessage(side));
        }
      } catch (err) {
        if (!alive) return;
        const msg = errorText(err);
        if (/NotAllowed|Permission|Denied/i.test(msg)) {
          setError("Akses kamera ditolak — izinkan di pengaturan browser atau gunakan input manual di bawah.");
        } else if (/NotFound|DevicesNotFound|Overconstrained/i.test(msg)) {
          setError("Kamera tidak ditemukan — gunakan input manual di bawah.");
        } else {
          setError("Kamera tidak dapat dibuka — gunakan input manual di bawah.");
        }
        setStatus("error");
      }
    });
    cameraChain.current = run.catch(() => undefined);

    return () => {
      alive = false;
      // Antre setelah pembukaan yang mungkin masih berjalan; ia menutup sendiri bila belum selesai.
      cameraChain.current = cameraChain.current.then(() => stopCamera(controls));
    };
  }, [request, handleFrame]);

  // Webcam USB dicabut/dipasang: segarkan ketersediaan tombol ganti kamera.
  useEffect(() => {
    const media = typeof navigator !== "undefined" ? navigator.mediaDevices : undefined;
    if (typeof media?.addEventListener !== "function") return;
    let alive = true;
    const onChange = () => {
      void listVideoInputs().then((list) => {
        if (alive) setCameras(classifyCameras(list, settingsRef.current));
      });
    };
    media.addEventListener("devicechange", onChange);
    return () => {
      alive = false;
      media.removeEventListener("devicechange", onChange);
    };
  }, []);

  const switchTarget = nextCamera(cameras, active);

  function handleSwitch() {
    if (!switchTarget || status === "loading") return;
    setStatus("loading");
    setAnnounce("");
    setRequest(switchTarget);
  }

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
        <video
          ref={videoRef}
          className={cn("size-full object-cover", active.side === "user" && "-scale-x-100")}
          muted
          playsInline
        />
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
        {switchTarget && (
          <button
            type="button"
            aria-label={switchCameraLabel(switchTarget)}
            disabled={status === "loading"}
            onClick={handleSwitch}
            className="absolute top-3 right-3 flex size-12 touch-manipulation items-center justify-center rounded-full bg-foreground/60 text-background select-none active:bg-foreground/80 disabled:opacity-50"
          >
            <SwitchCamera aria-hidden className="size-6" />
          </button>
        )}
        <p role="status" className="sr-only">
          {announce}
        </p>
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
