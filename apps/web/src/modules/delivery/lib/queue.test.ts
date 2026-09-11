import { describe, expect, it } from "vitest";
import { queueAgeLabel, queueAgeTone } from "@/modules/delivery/lib/queue";

describe("queueAgeLabel", () => {
  it("hari ini untuk 0 dan negatif", () => {
    expect(queueAgeLabel(0)).toBe("Hari ini");
    expect(queueAgeLabel(-2)).toBe("Hari ini");
  });

  it("n hari selebihnya", () => {
    expect(queueAgeLabel(1)).toBe("1 hari");
    expect(queueAgeLabel(8)).toBe("8 hari");
  });
});

describe("queueAgeTone", () => {
  it("merah di atas 7 hari, kuning di atas 0, netral hari ini", () => {
    expect(queueAgeTone(0)).toBe("neutral");
    expect(queueAgeTone(3)).toBe("warning");
    expect(queueAgeTone(8)).toBe("danger");
  });
});
