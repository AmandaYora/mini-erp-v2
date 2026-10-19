import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  Checkbox,
  ConfirmDialog,
  DataTable,
  FormField,
  FormSheet,
  Pagination,
  SearchInput,
  SearchSelect,
  SectionCard,
  StatusBadge,
  TextInput,
} from "@/shared/components/ui";
import { ActionMenu, Toolbar } from "@/shared/components/patterns";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { stockService } from "@/modules/stock/services/stock.service";
import { locationCreateSchema, locationUpdateSchema } from "@/modules/stock/schemas/stock.schema";
import { isParentHasStock, locationKindLabel, stripLocationCode } from "@/modules/stock/lib/location-guard";
import { clampPage, paginate } from "@/modules/stock/lib/paginate";
import { toId } from "@/modules/stock/lib/ids";
import type { StockLocation } from "@/modules/stock/types";

const LIMIT = 20;

type LocationMode = { kind: "create" } | { kind: "edit"; location: StockLocation };

interface Props {
  locations: StockLocation[];
  loading: boolean;
  error: string | null;
  onReload: () => void;
  canManage: boolean;
}

/** Tab Lokasi: gudang/rak cabang aktif. CRUD lewat FormSheet (T4), paginasi sisi klien. */
export function StockLocationTab({ locations, loading, error, onReload, canManage }: Props) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const [mode, setMode] = useState<LocationMode | null>(null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [parent, setParent] = useState<number | null>(null);
  const [isPicking, setIsPicking] = useState(false);
  const [isPrimary, setIsPrimary] = useState(false);
  const [sortOrder, setSortOrder] = useState("0");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  // Konfirmasi pemindahan stok induk (forceTransfer) setelah server menolak.
  const [forcePrompt, setForcePrompt] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<StockLocation | null>(null);

  const locationMap = useMemo(() => new Map(locations.map((l) => [l.id, l])), [locations]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (q === "") return locations;
    return locations.filter((l) => `${l.code} ${l.name}`.toLowerCase().includes(q));
  }, [locations, search]);
  const current = clampPage(page, filtered.length, LIMIT);
  const rows = paginate(filtered, current, LIMIT);

  function openCreate() {
    setCode("");
    setName("");
    setParent(null);
    setIsPicking(false);
    setIsPrimary(false);
    setSortOrder("0");
    setErrors({});
    setMode({ kind: "create" });
  }

  function openEdit(loc: StockLocation) {
    setCode(loc.code);
    setName(loc.name);
    setParent(loc.parentId);
    setIsPicking(loc.isPickingArea);
    setIsPrimary(loc.isPrimary);
    setSortOrder(String(loc.sortOrder));
    setErrors({});
    setMode({ kind: "edit", location: loc });
  }

  // Urutan kosong dibaca 0; teks bukan angka dibiarkan NaN agar skema menolaknya.
  function sortValue(): number {
    const t = sortOrder.trim();
    return t === "" ? 0 : Number(t);
  }

  function fieldErrors(issues: { path: PropertyKey[]; message: string }[]) {
    const errs: Record<string, string> = {};
    for (const issue of issues) errs[String(issue.path[0] ?? "")] ??= issue.message;
    return errs;
  }

  async function save(forceTransfer: boolean) {
    if (!mode) return;
    if (mode.kind === "create") {
      const parsed = locationCreateSchema.safeParse({
        code: code.trim().toUpperCase(),
        name: name.trim(),
        parentId: parent,
        forceTransfer: forceTransfer || undefined,
        isPickingArea: isPicking,
        isPrimary,
        sortOrder: sortValue(),
      });
      if (!parsed.success) {
        setErrors(fieldErrors(parsed.error.issues));
        return;
      }
      setErrors({});
      setSaving(true);
      try {
        const res = await stockService.createLocation(parsed.data);
        toast.fromServer(res.message, "Lokasi dibuat");
        setMode(null);
        onReload();
      } catch (err) {
        reportError(err);
      } finally {
        setSaving(false);
      }
      return;
    }
    const parsed = locationUpdateSchema.safeParse({
      // Kode hanya dikirim bila berubah (lokasi sistem tidak boleh berganti kode).
      code: code.trim().toUpperCase() !== mode.location.code ? code.trim().toUpperCase() : undefined,
      name: name.trim(),
      parentId: parent,
      forceTransfer: forceTransfer || undefined,
      isPickingArea: isPicking,
      isPrimary,
      sortOrder: sortValue(),
    });
    if (!parsed.success) {
      setErrors(fieldErrors(parsed.error.issues));
      return;
    }
    setErrors({});
    setSaving(true);
    try {
      const res = await stockService.updateLocation(mode.location.id, parsed.data);
      toast.fromServer(res.message, "Lokasi disimpan");
      setMode(null);
      onReload();
    } catch (err) {
      reportError(err);
    } finally {
      setSaving(false);
    }
  }

  function reportError(err: unknown) {
    const msg = toApiError(err).message;
    if (isParentHasStock(msg)) {
      setForcePrompt(stripLocationCode(msg));
      return;
    }
    toast.danger("Gagal menyimpan lokasi", msg);
  }

  async function handleArchive() {
    if (!archiveTarget) return;
    const target = archiveTarget;
    setArchiveTarget(null);
    try {
      const res = await stockService.archiveLocation(target.id);
      toast.fromServer(res.message, "Lokasi diarsipkan");
      onReload();
    } catch (err) {
      toast.danger("Gagal mengarsipkan lokasi", toApiError(err).message);
    }
  }

  const parentOptions = locations
    .filter((l) => (mode?.kind === "edit" ? l.id !== mode.location.id : true))
    .map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` }));

  const isEdit = mode?.kind === "edit";
  // Lokasi sistem (GDG/RUSAK) menjaga kode, nama, dan induknya; atributnya masih bisa diatur.
  const locationSystem = mode?.kind === "edit" && mode.location.isSystem;
  const attributesLocked = mode?.kind === "edit" && mode.location.kind === "damaged";

  return (
    <div className="flex flex-col gap-4">
      <Toolbar
        search={
          <SearchInput
            aria-label="Cari lokasi"
            placeholder="Cari kode atau nama lokasi…"
            value={search}
            onSearch={(v) => {
              setSearch(v);
              setPage(1);
            }}
          />
        }
        right={
          canManage ? (
            <Button size="sm" onClick={openCreate}>
              Tambah lokasi
            </Button>
          ) : undefined
        }
      />
      <SectionCard flush>
        <DataTable<StockLocation>
          loading={loading}
          error={error}
          onRetry={onReload}
          rows={rows}
          rowKey={(r) => r.id}
          emptyTitle={search.trim() ? "Tidak ada lokasi yang cocok" : "Belum ada lokasi"}
          emptyDescription={
            search.trim()
              ? "Coba kata kunci lain."
              : "Tambah lokasi penyimpanan untuk mulai mencatat stok."
          }
          emptyAction={
            canManage && !search.trim() ? <Button onClick={openCreate}>Tambah lokasi pertama</Button> : undefined
          }
          columns={[
            { header: "Kode", render: (r) => <span className="font-mono text-sm font-medium">{r.code}</span> },
            { header: "Nama", render: (r) => r.name },
            {
              header: "Jenis",
              render: (r) => (
                <Badge tone={r.kind === "damaged" ? "danger" : "neutral"}>{locationKindLabel(r.kind)}</Badge>
              ),
            },
            {
              header: "Induk",
              render: (r) =>
                r.parentId === null ? "–" : (locationMap.get(r.parentId)?.name ?? `#${r.parentId}`),
            },
            {
              header: "Alokasi",
              render: (r) =>
                r.kind === "damaged" ? (
                  <span className="text-xs text-muted-foreground">Tidak dialokasikan</span>
                ) : (
                  <span className="inline-flex flex-wrap items-center gap-1">
                    {r.isPickingArea && <Badge tone="info">Area picking</Badge>}
                    {r.isPrimary && <Badge tone="success">Utama</Badge>}
                    <span className="text-xs text-muted-foreground tabular">urutan {r.sortOrder}</span>
                  </span>
                ),
            },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
            ...(canManage
              ? [
                  {
                    header: "",
                    id: "actions",
                    align: "right" as const,
                    width: "3rem",
                    render: (r: StockLocation) => (
                      <ActionMenu
                        variant="ghost"
                        label={`Aksi untuk lokasi ${r.code}`}
                        items={[
                          { key: "edit", label: "Ubah", onSelect: () => openEdit(r) },
                          {
                            key: "archive",
                            label: r.isSystem ? "Arsipkan (lokasi sistem)" : "Arsipkan",
                            disabled: r.isSystem,
                            destructive: true,
                            onSelect: () => setArchiveTarget(r),
                          },
                        ]}
                      />
                    ),
                  },
                ]
              : []),
          ]}
        />
        {filtered.length > 0 && (
          <Pagination page={current} limit={LIMIT} total={filtered.length} onPageChange={setPage} />
        )}
      </SectionCard>

      <FormSheet
        open={mode !== null}
        title={isEdit ? "Ubah lokasi" : "Tambah lokasi"}
        description={
          isEdit
            ? "Kode lokasi dapat diubah selama tidak bentrok dengan lokasi lain di cabang ini."
            : "Gudang atau rak penyimpanan cabang aktif."
        }
        onClose={() => {
          if (!saving) setMode(null);
        }}
        actions={
          <>
            <Button variant="secondary" disabled={saving} onClick={() => setMode(null)}>
              Batal
            </Button>
            <Button type="submit" form="location-form" loading={saving}>
              Simpan
            </Button>
          </>
        }
      >
        <form
          id="location-form"
          className="flex flex-col gap-4"
          onSubmit={(e: FormEvent<HTMLFormElement>) => {
            e.preventDefault();
            void save(false);
          }}
        >
          <FormField label="Kode" htmlFor="location-code" required errorText={errors.code}>
            <TextInput
              id="location-code"
              value={code}
              disabled={isEdit && locationSystem}
              placeholder="cth: GDG-01"
              onChange={(e) => setCode(e.target.value.toUpperCase())}
            />
          </FormField>
          <FormField label="Nama" htmlFor="location-name" required errorText={errors.name}>
            <TextInput
              id="location-name"
              value={name}
              placeholder="cth: Gudang Utama"
              disabled={isEdit && locationSystem}
              onChange={(e) => setName(e.target.value)}
            />
          </FormField>
          <FormField label="Lokasi induk" errorText={errors.parentId}>
            <SearchSelect
              options={parentOptions}
              value={parent}
              placeholder="Tanpa induk"
              allowClear
              onChange={(v) => setParent(toId(v))}
              disabled={isEdit && locationSystem}
            />
          </FormField>
          <fieldset className="flex flex-col gap-3 rounded-md border border-border p-3" disabled={attributesLocked}>
            <legend className="px-1 text-sm font-medium text-foreground">Prioritas alokasi stok</legend>
            <p className="text-xs text-muted-foreground">
              Saran lokasi dan alokasi surat jalan mengambil dari: area picking, lalu lokasi utama, lalu gudang
              default, lalu stok terbanyak, lalu urutan terkecil.
            </p>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={isPicking} onCheckedChange={(v) => setIsPicking(v === true)} aria-label="Area picking" />
              Area picking (diambil lebih dulu)
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={isPrimary} onCheckedChange={(v) => setIsPrimary(v === true)} aria-label="Lokasi utama" />
              Lokasi utama
            </label>
            <FormField label="Urutan" htmlFor="location-sort" errorText={errors.sortOrder}>
              <TextInput
                id="location-sort"
                inputMode="numeric"
                value={sortOrder}
                onChange={(e) => setSortOrder(e.target.value)}
              />
            </FormField>
          </fieldset>
        </form>
      </FormSheet>

      <ConfirmDialog
        open={forcePrompt !== null}
        title="Pindahkan stok induk?"
        message={`${forcePrompt ?? ""} Lanjutkan dan pindahkan seluruh stok induk ke lokasi ini?`}
        confirmLabel="Pindahkan dan simpan"
        tone="primary"
        onConfirm={() => {
          setForcePrompt(null);
          void save(true);
        }}
        onCancel={() => setForcePrompt(null)}
      />

      <ConfirmDialog
        open={archiveTarget !== null}
        title="Arsipkan lokasi"
        message={`Arsipkan lokasi ${archiveTarget?.code ?? ""} — ${archiveTarget?.name ?? ""}? Lokasi arsip tidak bisa dipakai transaksi baru.`}
        confirmLabel="Arsipkan"
        onConfirm={() => void handleArchive()}
        onCancel={() => setArchiveTarget(null)}
      />
    </div>
  );
}
