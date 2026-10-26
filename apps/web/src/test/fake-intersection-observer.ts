import { act } from "@testing-library/react";
import { expect } from "vitest";

/** IntersectionObserver palsu untuk pemilih berhalaman: jsdom tidak punya layout,
 * jadi tes memicu "sentinel terlihat" (gulir mendekati bawah daftar) sendiri. */
export class FakeIntersectionObserver {
  static live: FakeIntersectionObserver[] = [];
  constructor(private readonly cb: IntersectionObserverCallback) {
    FakeIntersectionObserver.live.push(this);
  }
  observe() {}
  unobserve() {}
  takeRecords() {
    return [];
  }
  disconnect() {
    FakeIntersectionObserver.live = FakeIntersectionObserver.live.filter((o) => o !== this);
  }
  fire() {
    this.cb([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver);
  }
}

/** Gulir daftar pemilih sampai bawah (memicu muat halaman berikutnya). */
export function scrollPickerToEnd() {
  const observer = FakeIntersectionObserver.live.at(-1);
  expect(observer).toBeDefined();
  act(() => observer?.fire());
}
