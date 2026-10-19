import { useState } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Numpad, type NumpadProps } from "@/shared/components/patterns";

afterEach(cleanup);
// Banyak ketukan userEvent berurutan: beri ruang saat suite dijalankan paralel.
vi.setConfig({ testTimeout: 20_000 });

function Harness({ initial = "", ...props }: { initial?: string } & Partial<NumpadProps>) {
  const [value, setValue] = useState(initial);
  return (
    <div>
      <output aria-label="nilai">{value}</output>
      <Numpad value={value} onChange={setValue} {...props} />
    </div>
  );
}

const shown = () => screen.getByLabelText("nilai").textContent;
const tap = async (...names: string[]) => {
  for (const n of names) await userEvent.click(screen.getByRole("button", { name: n }));
};

describe("Numpad", () => {
  it("mengetik angka dan 00", async () => {
    render(<Harness onEnter={() => {}} />);
    await tap("1", "2", "00");
    expect(shown()).toBe("1200");
  });

  it("00 dan 0 di awal tidak menumpuk nol", async () => {
    render(<Harness onEnter={() => {}} />);
    await tap("00");
    expect(shown()).toBe("");
    await tap("0", "00", "0");
    expect(shown()).toBe("0");
    await tap("5");
    expect(shown()).toBe("5");
  });

  it("hapus mundur satu angka dan aman pada nilai kosong", async () => {
    render(<Harness initial="123" onEnter={() => {}} />);
    await tap("Hapus angka terakhir", "Hapus angka terakhir");
    expect(shown()).toBe("1");
    await tap("Hapus angka terakhir", "Hapus angka terakhir");
    expect(shown()).toBe("");
  });

  it("tombol desimal hanya bila diizinkan, satu kali, dan awalan 0.", async () => {
    const { rerender } = render(<Harness onEnter={() => {}} />);
    expect(screen.queryByRole("button", { name: "Koma" })).not.toBeInTheDocument();
    rerender(<Harness allowDecimal onEnter={() => {}} />);
    await tap("Koma");
    expect(shown()).toBe("0.");
    await tap("5", "Koma", "2");
    expect(shown()).toBe("0.52");
  });

  it("label desimal titik bila diminta", () => {
    render(<Harness allowDecimal decimalSeparator="." onEnter={() => {}} />);
    expect(screen.getByRole("button", { name: "Titik" })).toBeInTheDocument();
  });

  it("menghormati maxLength", async () => {
    render(<Harness maxLength={3} onEnter={() => {}} />);
    await tap("1", "2", "3", "4", "00");
    expect(shown()).toBe("123");
  });

  it("Enter memanggil onEnter; tanpa onEnter tombol nonaktif", async () => {
    const onEnter = vi.fn();
    const { rerender } = render(<Harness onEnter={onEnter} enterLabel="Terapkan" />);
    await tap("Terapkan");
    expect(onEnter).toHaveBeenCalledTimes(1);
    rerender(<Harness enterLabel="Terapkan" />);
    expect(screen.getByRole("button", { name: "Terapkan" })).toBeDisabled();
  });

  it("disabled menonaktifkan semua tombol", () => {
    render(<Harness disabled onEnter={() => {}} />);
    for (const b of screen.getAllByRole("button")) expect(b).toBeDisabled();
  });

  it("semua tombol dapat dijangkau sebagai grup berlabel", () => {
    render(<Harness onEnter={() => {}} />);
    expect(screen.getByRole("group", { name: "Papan angka" })).toBeInTheDocument();
    expect(screen.getAllByRole("button")).toHaveLength(13);
  });
});
