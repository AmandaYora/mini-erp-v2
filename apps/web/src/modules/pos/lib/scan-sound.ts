// Bunyi sukses pindai (SL-08). Aset: public/sounds/scan-beep.wav (nada sinus
// pendek yang dibangkitkan sendiri — tanpa lisensi pihak ketiga). Preferensi
// bisu disimpan per perangkat; tanpa pilihan tersimpan, pengguna yang meminta
// gerak/efek dikurangi (prefers-reduced-motion) mendapat bawaan bisu.

export const SCAN_SOUND_SRC = "/sounds/scan-beep.wav";
const KEY = "scan-sound-muted";

export function isScanSoundMuted(): boolean {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "1") return true;
    if (v === "0") return false;
  } catch {
    /* penyimpanan tak tersedia: ikut bawaan */
  }
  try {
    return typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true;
  } catch {
    return false;
  }
}

export function setScanSoundMuted(muted: boolean): void {
  try {
    localStorage.setItem(KEY, muted ? "1" : "0");
  } catch {
    /* abaikan */
  }
}

/** Mainkan bunyi bila tidak dibisukan; kegagalan putar (kebijakan autoplay) diabaikan. */
export function playScanSound(): void {
  if (isScanSoundMuted()) return;
  try {
    const audio = new Audio(SCAN_SOUND_SRC);
    audio.volume = 0.6;
    void audio.play()?.catch(() => undefined);
  } catch {
    /* tanpa audio: diam */
  }
}
