import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SearchInput } from "./search-input";

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("SearchInput", () => {
  it("menunda onSearch sampai jeda ketik habis dan mengirim nilai ter-trim", () => {
    const onSearch = vi.fn();
    render(<SearchInput aria-label="cari" value="" onSearch={onSearch} debounceMs={400} />);
    const input = screen.getByLabelText("cari");
    fireEvent.change(input, { target: { value: "ab" } });
    fireEvent.change(input, { target: { value: " abc " } });
    act(() => vi.advanceTimersByTime(399));
    expect(onSearch).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(onSearch).toHaveBeenCalledTimes(1);
    expect(onSearch).toHaveBeenCalledWith("abc");
  });

  it("Enter langsung mengirim tanpa menunggu", () => {
    const onSearch = vi.fn();
    render(<SearchInput aria-label="cari" value="" onSearch={onSearch} />);
    const input = screen.getByLabelText("cari");
    fireEvent.change(input, { target: { value: "xyz" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSearch).toHaveBeenCalledWith("xyz");
    act(() => vi.advanceTimersByTime(1000));
    expect(onSearch).toHaveBeenCalledTimes(1);
  });

  it("tidak mengirim ulang bila nilai tidak berubah", () => {
    const onSearch = vi.fn();
    render(<SearchInput aria-label="cari" value="abc" onSearch={onSearch} />);
    fireEvent.blur(screen.getByLabelText("cari"));
    expect(onSearch).not.toHaveBeenCalled();
  });

  it("mengikuti perubahan nilai dari luar (reset)", () => {
    const { rerender } = render(<SearchInput aria-label="cari" value="abc" onSearch={vi.fn()} />);
    expect(screen.getByLabelText("cari")).toHaveValue("abc");
    rerender(<SearchInput aria-label="cari" value="" onSearch={vi.fn()} />);
    expect(screen.getByLabelText("cari")).toHaveValue("");
  });
});
