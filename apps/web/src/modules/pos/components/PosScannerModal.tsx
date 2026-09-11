import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  FormField,
  Modal,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  cameraFallbackMessage,
  cameraUnavailableReason,
} from "@/modules/pos/lib/camera";

// Pindai kamera QR/barcode via @zxing/browser (dynamic import agar bundle awal
// tetap kecil). Kamera butuh HTTPS + izin; bila tak tersedia / ditolak, input
// ketik/wedge di bawah selalu tampil sebagai fallback (tak pernah gagal total).
interface ScannerControls {
  stop: () => void;
}

interface DecodeResult {
  getText: () => string;
}

export function PosScannerModal({
  open,
  onClose,
  onScan,
}: {
  open: boolean;
  onClose: () => void;
  onScan: (code: string) => void;
}) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const controlsRef = useRef<ScannerControls | null>(null);
  const onScanRef = useRef(onScan);
  useEffect(() => {
    onScanRef.current = onScan;
  }, [onScan]);

  const [devices, setDevices] = useState<{ deviceId: string; label: string }[]>([]);
  const [deviceId, setDeviceId] = useState<string>("");
  const [status, setStatus] = useState<"loading" | "scanning" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const [manual, setManual] = useState("");

  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setStatus("loading");
      setError(null);
      setManual("");
      setDevices([]);
      setDeviceId("");
    }
  }

  useEffect(() => {
    if (!open) return;

    let alive = true;
    let controls: ScannerControls | null = null;

    void (async () => {
      const reason = cameraUnavailableReason();
      if (reason) {
        if (!alive) return;
        setError(cameraFallbackMessage(reason));
        setStatus("error");
        return;
      }
      try {
        const { BrowserMultiFormatReader } = await import("@zxing/browser");
        if (!alive) return;
        const reader = new BrowserMultiFormatReader();
        const list: { deviceId: string; label: string }[] =
          await BrowserMultiFormatReader.listVideoInputDevices();
        if (!alive) return;
        setDevices(list.map((d) => ({ deviceId: d.deviceId, label: d.label })));
        const picked = list[list.length - 1]?.deviceId ?? list[0]?.deviceId ?? "";
        setDeviceId(picked);
        const video = videoRef.current;
        if (!video) {
          setError("Pratinjau kamera tidak tersedia — gunakan input manual di bawah.");
          setStatus("error");
          return;
        }
        controls = await reader.decodeFromVideoDevice(
          picked || undefined,
          video,
          (result: DecodeResult | undefined, err: unknown) => {
            if (!alive) return;
            if (result) {
              const text = result.getText().trim();
              if (text !== "") {
                onScanRef.current(text);
              }
            } else if (err instanceof Error) {
              // NotFoundException per frame = belum ada kode di bidikan; abaikan.
              if (!/NotFound/i.test(err.name)) {
                setError(`Kamera gagal membaca (${err.name}) — gunakan input manual di bawah.`);
                setStatus("error");
              }
            }
          },
        );
        controlsRef.current = controls;
        if (!alive) {
          controls.stop();
          controlsRef.current = null;
          return;
        }
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
      controlsRef.current?.stop();
      controlsRef.current = null;
    };
  }, [open ]);

  // Ganti kamera: hentikan stream lama lalu mulai ulang dengan deviceId baru.
  useEffect(() => {
    if (!open || status !== "scanning" || deviceId === "") return;
    const video = videoRef.current;
    if (!video) return;
    let alive = true;
    let controls: ScannerControls | null = null;
    controlsRef.current?.stop();
    controlsRef.current = null;
    void (async () => {
      try {
        const { BrowserMultiFormatReader } = await import("@zxing/browser");
        if (!alive) return;
        const reader = new BrowserMultiFormatReader();
        controls = await reader.decodeFromVideoDevice(deviceId, video, (result: DecodeResult | undefined) => {
          if (!alive || !result) return;
          const text = result.getText().trim();
          if (text !== "") onScanRef.current(text);
        });
        controlsRef.current = controls;
      } catch {
        if (alive) setError("Gagal berganti kamera — gunakan input manual di bawah.");
      }
    })();
    return () => {
      alive = false;
      controls?.stop();
    };
  }, [open, status, deviceId]);

  function handleManual(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const code = manual.trim();
    if (code === "") return;
    onScan(code);
  }

  return (
    <Modal
      open={open}
      title="Scan Barcode / QR"
      description="Arahkan kamera ke barcode produk, atau ketik manual."
      size="sm"
      onClose={onClose}
      actions={
        <Button variant="secondary" onClick={onClose}>
          Tutup
        </Button>
      }
    >
      <div className="space-y-4">
        {status === "loading" && (
          <p className="text-muted py-4 text-center text-sm">Membuka kamera…</p>
        )}
        {status !== "loading" && (
          <div className="overflow-hidden rounded-md border border-hairline bg-surface-subtle">
            <video ref={videoRef} className="max-h-[300px] w-full" muted playsInline />
          </div>
        )}
        {devices.length >= 2 && (
          <FormField label="Kamera" htmlFor="pos-scan-device">
            <SelectInput
              id="pos-scan-device"
              value={deviceId}
              onChange={(e) => setDeviceId(e.target.value)}
            >
              {devices.map((d, i) => (
                <option key={d.deviceId} value={d.deviceId}>
                  {d.label || `Kamera ${i + 1}`}
                </option>
              ))}
            </SelectInput>
          </FormField>
        )}
        {error && <Notice tone="warning" title={error} />}
        <form onSubmit={handleManual}>
          <FormField
            label="Kode manual"
            helperText="Untuk keyboard-wedge / scanner USB: fokuskan lalu pindai."
            htmlFor="pos-scan-manual"
          >
            <div className="flex gap-2">
              <TextInput
                id="pos-scan-manual"
                value={manual}
                onChange={(e) => setManual(e.target.value)}
                placeholder="Ketik / tempel barcode…"
                autoComplete="off"
              />
              <Button type="submit" disabled={manual.trim() === ""}>
                Tambah
              </Button>
            </div>
          </FormField>
        </form>
      </div>
    </Modal>
  );
}
