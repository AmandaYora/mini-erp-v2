import { cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TileThumb } from "@/modules/pos/components/ProductGrid";

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
