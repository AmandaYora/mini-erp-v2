// @vitest-environment node
// Cakupan label audit vs aksi/entitas yang BENAR-BENAR ditulis backend (U-02).
// Memindai sumber Go (non-test) di apps/api/internal/modules. Bila test ini merah:
//  - "tanpa label"  → backend menulis aksi/entitas baru; tambahkan di audit-labels.ts.
//  - "label yatim"  → label menunjuk kunci yang tak lagi ditulis siapa pun; hapus.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { ACTION_LABELS, ENTITY_LABELS, actionLabel, entityName } from "@/modules/audit/lib/audit-labels";

const MODULES_DIR = resolve(process.cwd(), "../api/internal/modules");

function goFiles(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) goFiles(p, out);
    else if (p.endsWith(".go") && !p.endsWith("_test.go")) out.push(p);
  }
  return out;
}

const KEY = String.raw`([a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+)`;
const ENT = String.raw`([a-z][a-z0-9_]*)`;

/**
 * Bentuk penulisan aksi di backend:
 *  1. Entry{Action: "x.y", Entity: "e"}               (literal; mayoritas)
 *  2. s.trail/trailDiff(ctx, "x.y", "entity", ...)    (assistant)
 *  3. s.periodAudit("x.y", ...)                       (finance: periode)
 *  4. s.insertEntry(..., "x.y")                       (finance: jurnal; argumen terakhir)
 *  5. action, entity := "x.y", "entity"               (party: arsip/pulihkan)
 */
function scanBackend() {
  const actions = new Set<string>();
  const entities = new Set<string>();
  for (const file of goFiles(MODULES_DIR)) {
    const src = readFileSync(file, "utf8");
    for (const m of src.matchAll(new RegExp(String.raw`Action:\s*"${KEY}"`, "g"))) actions.add(m[1]);
    for (const m of src.matchAll(new RegExp(String.raw`Entity:\s*"${ENT}"`, "g"))) entities.add(m[1]);
    for (const m of src.matchAll(new RegExp(String.raw`\btrail(?:Diff)?\(\s*ctx,\s*"${KEY}",\s*"${ENT}"`, "g"))) {
      actions.add(m[1]);
      entities.add(m[2]);
    }
    for (const m of src.matchAll(new RegExp(String.raw`periodAudit\(\s*"${KEY}"`, "g"))) actions.add(m[1]);
    for (const m of src.matchAll(new RegExp(String.raw`insertEntry\w*\((?:[^()]|\([^()]*\))*?,\s*"${KEY}"[,)]`, "g"))) {
      actions.add(m[1]);
    }
    for (const m of src.matchAll(new RegExp(String.raw`action,\s*entity\s*:?=\s*"${KEY}",\s*"${ENT}"`, "g"))) {
      actions.add(m[1]);
      entities.add(m[2]);
    }
  }
  return { actions, entities };
}

describe("label audit vs backend", () => {
  const { actions, entities } = scanBackend();

  it("pemindaian menemukan aksi backend (jaga agar pemindai tidak diam-diam kosong)", () => {
    expect(actions.size).toBeGreaterThan(80);
    expect(entities.size).toBeGreaterThan(25);
    for (const known of ["users.create", "sales_orders.confirm", "journals.post", "fiscal_periods.reopen", "customers.archive"]) {
      expect(actions.has(known), known).toBe(true);
    }
  });

  it("setiap aksi yang ditulis backend punya label Indonesia", () => {
    const missing = [...actions].filter((a) => !(a in ACTION_LABELS)).sort();
    expect(missing, `aksi tanpa label: ${missing.join(", ")}`).toEqual([]);
  });

  it("setiap entitas yang ditulis backend punya label Indonesia", () => {
    const missing = [...entities].filter((e) => !(e in ENTITY_LABELS)).sort();
    expect(missing, `entitas tanpa label: ${missing.join(", ")}`).toEqual([]);
  });

  it("tidak ada label yatim (kunci yang tak ditulis backend)", () => {
    const orphanA = Object.keys(ACTION_LABELS).filter((a) => !actions.has(a)).sort();
    const orphanE = Object.keys(ENTITY_LABELS).filter((e) => !entities.has(e)).sort();
    expect(orphanA, `label aksi yatim: ${orphanA.join(", ")}`).toEqual([]);
    expect(orphanE, `label entitas yatim: ${orphanE.join(", ")}`).toEqual([]);
  });

  it("label tidak kosong dan bukan kunci mentah", () => {
    for (const [k, v] of Object.entries({ ...ACTION_LABELS, ...ENTITY_LABELS })) {
      expect(v.trim(), k).not.toBe("");
      expect(v, k).not.toBe(k);
      expect(v, k).not.toMatch(/^[a-z_]+\.[a-z_]+$/);
    }
  });

  it("kunci tak dikenal jatuh ke kunci mentah", () => {
    expect(actionLabel("job.sweep")).toBe("job.sweep");
    expect(entityName("entitas_lama")).toBe("entitas_lama");
    expect(actionLabel("users.create")).toBe("Tambah pengguna");
  });
});
