import { cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TileImage, TileThumb } from "@/modules/pos/components/ProductGrid";

afterEach(cleanup);

describe("TileThumb (temuan 4: foto produk di tile POS)", () => {
  it("menampilkan foto lazy bila ada URL", () => {
    const { container } = render(<TileThumb name="Semen Tiga Roda" src="/media/abc.webp?exp=1&sig=x" />);
    const img = container.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/media/abc.webp?exp=1&sig=x");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(container.textContent).toBe("");
  });

  it("tanpa foto atau foto gagal dimuat: kembali ke inisial", () => {
    const none = render(<TileThumb name="Semen Tiga Roda" src={null} />);
    expect(none.container.textContent).toBe("ST");
    cleanup();
    const { container } = render(<TileThumb name="Besi Beton" src="/media/kedaluwarsa.webp" />);
    fireEvent.error(container.querySelector("img")!);
    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toBe("BB");
  });
});

describe("TileImage (analysis/003: foto utama kartu katalog)", () => {
  it("foto 4:3 selebar kartu, lazy, ruang dipesan lebih dulu", () => {
    const { container } = render(<TileImage name="Semen Tiga Roda" src="/media/abc.webp" />);
    const box = container.firstElementChild as HTMLElement;
    expect(box.className).toContain("aspect-[4/3]");
    expect(box.className).toContain("w-full");
    const img = container.querySelector("img");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(img?.className).toContain("object-cover");
    expect(img?.getAttribute("alt")).toBe("");
  });

  it("tanpa foto / gagal dimuat: inisial di kotak 4:3 yang sama", () => {
    const { container } = render(<TileImage name="Besi Beton" src="/media/x.webp" />);
    fireEvent.error(container.querySelector("img")!);
    const box = container.firstElementChild as HTMLElement;
    expect(box.textContent).toBe("BB");
    expect(box.className).toContain("aspect-[4/3]");
  });

  it("stok habis meredupkan foto", () => {
    const { container } = render(<TileImage name="Cat" src="/media/c.webp" dimmed />);
    expect(container.querySelector("img")?.className).toContain("grayscale");
  });
});

