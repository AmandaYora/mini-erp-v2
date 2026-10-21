import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import {
  Button,
  Checkbox,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  DateInput,
  FormField,
  PageHeader,
  SearchSelect,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { LineItemsEditor, Page, Qty, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { ProductSearchSelect } from "@/modules/products/components/ProductSearchSelect";
import { customerService } from "@/modules/party/services/party.service";
import type { Address, Party } from "@/modules/party/types";
import type { ProductOptionRow } from "@/modules/products/types";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import {
  directDeliverySchema,
  toDirectDeliveryPayload,
} from "@/modules/delivery/schemas/direct-delivery.schema";

// Halaman Buat SJ Langsung (T3, ADR-0022): surat jalan untuk pelanggan
// terdaftar TANPA order — order dibuat belakangan dari SJ ini oleh admin
// penjualan. Rute `/deliveries/new-direct`, izin delivery.create_direct.
// Tanpa harga: baris hanya produk berstok (satuan dari master produk), qty,
// dan lokasi opsional (kosong = server mengalokasikan lintas lokasi).

interface DirectLineRow {
  key: number;
  productId: number;
  productLabel: string;
  variantId: number;
  variants: { id: number; code: string; name: string }[];
  uom: string;
  uoms: string[];
  qty: string;
  locationId: number | null;
}

interface ShipAdHoc {
  on: boolean;
  label: string;
  recipient: string;
  phone: string;
  address: string;
}

const EMPTY_AD_HOC: ShipAdHoc = { on: false, label: "", recipient: "", phone: "", address: "" };

let nextLineKey = 1;

function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `sjl-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

function toId(v: string | number | null): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

/** Satuan yang dikenal master produk (jual, beli, dasar) tanpa duplikat. */
function uniqueUoms(p: ProductOptionRow): string[] {
  return [p.salesUom, p.purchaseUom, p.baseUom].filter(
    (u, i, arr) => u.trim() !== "" && arr.indexOf(u) === i,
  );
}

export default function CreateDirectDeliveryPage() {
  const navigate = useNavigate();

  const [partyId, setPartyId] = useState<number | null>(null);
  const [pickedParty, setPickedParty] = useState<Party | null>(null);
  const [shipToAddressId, setShipToAddressId] = useState<number | null>(null);
  const [shipAdHoc, setShipAdHoc] = useState<ShipAdHoc>(EMPTY_AD_HOC);
  const [deliveryDate, setDeliveryDate] = useState(todayWIB());
  const [driverName, setDriverName] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [warehouseStaffName, setWarehouseStaffName] = useState("");
  const [dropLocationNote, setDropLocationNote] = useState("");
  const [notes, setNotes] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);
  const [lines, setLines] = useState<DirectLineRow[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  // Galat per kolom (Zod klien atau errors[] server).
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  // Satu UUID per halaman dibuka: klik ganda/retry = replay server, bukan SJ ganda.
  const [clientRef] = useState(newClientRef);

  // Buku alamat pelanggan (addressView: id, label, recipient, phone, text, isPrimary).
  const { data: addrRows, loading: addrLoadingRaw } = useAsyncData(
    () =>
      partyId === null
        ? Promise.resolve([] as Address[])
        : customerService.listAddresses(partyId).then(
            (rows) => rows,
            (): Address[] => [],
          ),
    [partyId],
  );
  const shipAddresses = partyId === null ? [] : (addrRows ?? []);
  const addrLoading = partyId === null ? false : addrLoadingRaw;

  // Peringatan lunak: pelanggan masih punya SJ Langsung yang belum diorder
  // (mencegah barang keluar dua kali; angka dari server, best-effort).
  const { data: openSummary } = useAsyncData(
    (signal) =>
      partyId === null
        ? Promise.resolve(null)
        : deliveryService.openDirect({ partyId, limit: 0 }, signal).catch(() => null),
    [partyId],
  );

  const { data: locations } = useAsyncData(
    () =>
      deliveryService.locationOptions().catch((err) => {
        toast.danger("Gagal memuat lokasi", toApiError(err).message);
        return [];
      }),
    [],
  );

  function patchLine(key: number, patch: Partial<DirectLineRow>) {
    setLines((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  /** Tambah baris dari opsi pemilih (search-options sudah membawa varian & satuan). */
  function addLine(d: ProductOptionRow) {
    if (d.type !== "barang" || !d.tracked) {
      toast.warning(
        "Bukan barang berstok",
        `${d.code} — ${d.name} adalah jasa/non-stok; tambahkan di order, bukan di surat jalan langsung.`,
      );
      return;
    }
    const activeVariants = d.variants.filter((v) => v.status === "active");
    const variants = (activeVariants.length > 0 ? activeVariants : d.variants).map((v) => ({
      id: v.id,
      code: v.code,
      name: v.name,
    }));
    if (variants.length === 0) {
      toast.danger("Gagal menambah baris", "Produk tidak punya varian.");
      return;
    }
    const uoms = uniqueUoms(d);
    if (uoms.length === 0) {
      toast.danger("Gagal menambah baris", "Produk tidak punya satuan.");
      return;
    }
    const defaultVariant = d.variants.find((v) => v.isDefault && variants.some((x) => x.id === v.id)) ?? variants[0];
    const uom = uoms.includes(d.salesUom) ? d.salesUom : uoms[0];
    if (lines.some((r) => r.productId === d.id && r.variantId === defaultVariant.id && r.uom === uom)) {
      toast.warning("Baris sudah ada", "Ubah qty pada baris yang sudah ada.");
      return;
    }
    setLines((rows) => [
      ...rows,
      {
        key: nextLineKey++,
        productId: d.id,
        productLabel: `${d.code} — ${d.name}`,
        variantId: defaultVariant.id,
        variants,
        uom,
        uoms,
        qty: "1",
        locationId: null,
      },
    ]);
    setFieldErrors((e) => {
      if (!e.items) return e;
      const rest = { ...e };
      delete rest.items;
      return rest;
    });
  }

  function removeLine(index: number) {
    setLines((rows) => rows.filter((_, i) => i !== index));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const adHocActive = shipAdHoc.on || shipAddresses.length === 0;
    const parsed = directDeliverySchema.safeParse({
      partyId: partyId ?? 0,
      deliveryDate,
      shipToAddressId: adHocActive ? 0 : (shipToAddressId ?? 0),
      shipToLabel: adHocActive ? shipAdHoc.label : "",
      shipToRecipient: adHocActive ? shipAdHoc.recipient : "",
      shipToPhone: adHocActive ? shipAdHoc.phone : "",
      shipToAddress: adHocActive ? shipAdHoc.address : "",
      driverName,
      vehiclePlate,
      warehouseStaffName,
      dropLocationNote,
      notes,
      items: lines.map((r) => ({
        productId: r.productId,
        variantId: r.variantId,
        locationId: r.locationId ?? 0,
        uom: r.uom,
        qty: r.qty,
      })),
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const key = issue.path.length > 0 ? String(issue.path[0]) : "_";
        mapped[key] ??= issue.message;
      }
      setFieldErrors(mapped);
      setFormError(parsed.error.issues[0]?.message ?? "Periksa kembali isian formulir.");
      if (mapped.shipToAddress || mapped.shipToLabel || mapped.shipToRecipient || mapped.shipToPhone) setDetailOpen(true);
      return;
    }
    setFormError(null);
    setFieldErrors({});
    setSaving(true);
    try {
      const res = await deliveryService.createDirect(toDirectDeliveryPayload(parsed.data, clientRef));
      // Replay (HTTP 200, replayed:true): SJ sudah ada — pesan server dipakai apa adanya.
      toast.fromServer(res.message, "Surat jalan langsung dibuat", res.data.number);
      const warnings = (res.data.warnings ?? []).filter(Boolean);
      for (const w of warnings) toast.warning("Perhatian", w);
      navigate(`${ROUTE_PATHS.deliveries}/${res.data.id}`, warnings.length > 0 ? { state: { warnings } } : undefined);
    } catch (err) {
      const apiErr = toApiError(err);
      setFormError(apiErr.message);
      const mapped: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) mapped[fe.field] ??= fe.message;
      setFieldErrors(mapped);
      if (mapped.shipToAddress || mapped.shipToAddressId) setDetailOpen(true);
      toast.danger("Gagal membuat surat jalan", apiErr.message);
    } finally {
      setSaving(false);
    }
  }

  const totalQty = lines.reduce((sum, r) => sum + (Number(r.qty) || 0), 0);
  const errorCount = Object.keys(fieldErrors).length;
  const pickedAddress =
    shipToAddressId !== null ? shipAddresses.find((a) => a.id === shipToAddressId) ?? null : null;
  const adHocVisible = partyId !== null && (shipAdHoc.on || (!addrLoading && shipAddresses.length === 0));

  return (
    <Page>
      <PageHeader
        title="Buat SJ Langsung"
        description="Surat jalan tanpa order untuk pelanggan terdaftar; order jual dibuat belakangan dari surat jalan ini. Tanpa harga. Draf tidak menggerakkan stok; stok keluar saat konfirmasi."
      />

      {formError && (
        <Notice tone="danger" title="Surat jalan belum tersimpan">
          {formError}
        </Notice>
      )}
      {openSummary && openSummary.count > 0 && (
        <Notice tone="warning" title="Pelanggan ini masih punya surat jalan langsung yang belum diorder">
          {openSummary.count} surat jalan menunggu order
          {openSummary.oldestDate ? ` (tertua ${formatDate(openSummary.oldestDate)})` : ""}. Pastikan barang
          tidak dikirim dua kali; buat ordernya dari menu Order Jual.
        </Notice>
      )}

      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="flex flex-col gap-6">
        <SectionCard title="Dokumen">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="sm:col-span-2">
              <FormField label="Pelanggan" required errorText={fieldErrors.partyId}>
                <PartySearchSelect
                  partyKind="customer"
                  value={partyId}
                  selectedParty={pickedParty}
                  onChange={(id, party) => {
                    setPartyId(id);
                    setPickedParty(party ?? null);
                    // Alamat milik pelanggan lama tak berlaku untuk pelanggan baru.
                    setShipToAddressId(null);
                    setShipAdHoc(EMPTY_AD_HOC);
                  }}
                  placeholder="Cari pelanggan (nama, kode, telepon)…"
                />
              </FormField>
            </div>
            <FormField label="Tanggal kirim" required helperText="Tidak boleh di masa depan." errorText={fieldErrors.deliveryDate}>
              <DateInput value={deliveryDate} max={todayWIB()} onChange={(e) => setDeliveryDate(e.target.value)} />
            </FormField>
          </div>

          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <FormField label="Supir" required errorText={fieldErrors.driverName}>
              <TextInput
                value={driverName}
                maxLength={100}
                onChange={(e) => setDriverName(e.target.value)}
                placeholder="Nama supir"
              />
            </FormField>
            <FormField label="Plat kendaraan" errorText={fieldErrors.vehiclePlate}>
              <TextInput
                value={vehiclePlate}
                maxLength={20}
                onChange={(e) => setVehiclePlate(e.target.value)}
                placeholder="cth. B 1234 CD (opsional)"
              />
            </FormField>
            <FormField label="Petugas gudang" required errorText={fieldErrors.warehouseStaffName}>
              <TextInput
                value={warehouseStaffName}
                maxLength={100}
                onChange={(e) => setWarehouseStaffName(e.target.value)}
                placeholder="Nama petugas gudang"
              />
            </FormField>
          </div>
          <p className="mt-3 text-xs text-muted-foreground">
            Surat jalan langsung mengikuti aturan bukti kirim yang sama: nama sopir dan petugas gudang tercantum,
            dan saat konfirmasi nama penerima, status tanda tangan, serta foto surat jalan bertanda tangan wajib
            (kecuali perusahaan menonaktifkannya). Sopir dan petugas tidak bisa diubah setelah draf disimpan.
          </p>

          <Collapsible open={detailOpen} onOpenChange={setDetailOpen} className="mt-4 border-t border-border pt-3">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="-ml-2">
                <ChevronRight
                  aria-hidden
                  className={detailOpen ? "rotate-90 transition-transform" : "transition-transform"}
                />
                Alamat kirim &amp; detail lain
                <span className="font-normal text-muted-foreground">(alamat kirim, catatan turun barang, catatan)</span>
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-3">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                {partyId === null && (
                  <p className="text-sm text-muted-foreground sm:col-span-2 lg:col-span-4">
                    Pilih pelanggan dulu untuk mengisi alamat kirim.
                  </p>
                )}
                {partyId !== null && !shipAdHoc.on && shipAddresses.length > 0 && (
                  <div className="sm:col-span-2 lg:col-span-4">
                    <FormField
                      label="Alamat kirim"
                      helperText={addrLoading ? "Memuat buku alamat…" : "Kosongkan untuk memakai alamat pelanggan sendiri."}
                      errorText={fieldErrors.shipToAddressId}
                    >
                      <SearchSelect
                        aria-label="Alamat kirim"
                        options={shipAddresses.map((a) => ({
                          value: a.id,
                          label: `${a.label || "Alamat"} — ${a.recipient || "-"}${a.isPrimary ? " (Utama)" : ""}`,
                        }))}
                        value={shipToAddressId}
                        onChange={(v) => setShipToAddressId(toId(v))}
                        placeholder="Alamat pelanggan sendiri"
                        allowClear
                        disabled={addrLoading}
                      />
                    </FormField>
                    {pickedAddress && (
                      <p className="mt-1 text-xs text-muted-foreground">
                        {pickedAddress.text}
                        {[pickedAddress.recipient, pickedAddress.phone].filter((s) => s && s.trim() !== "").length > 0 && (
                          <> · {[pickedAddress.recipient, pickedAddress.phone].filter((s) => s && s.trim() !== "").join(" · ")}</>
                        )}
                      </p>
                    )}
                  </div>
                )}
                {partyId !== null && shipAddresses.length > 0 && (
                  <div className="sm:col-span-2 lg:col-span-4">
                    <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                      <Checkbox
                        id="f-ship-adhoc"
                        checked={shipAdHoc.on}
                        onCheckedChange={(v) => {
                          setShipAdHoc((a) => ({ ...a, on: v === true }));
                          if (v === true) setShipToAddressId(null);
                        }}
                      />
                      Pakai alamat sekali pakai (tidak dari buku alamat)
                    </label>
                  </div>
                )}
                {adHocVisible && (
                  <div className="grid grid-cols-1 gap-4 sm:col-span-2 sm:grid-cols-2 lg:col-span-4" data-testid="ship-adhoc">
                    <FormField label="Label" htmlFor="f-ship-label" errorText={fieldErrors.shipToLabel}>
                      <TextInput
                        id="f-ship-label"
                        value={shipAdHoc.label}
                        maxLength={120}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, label: e.target.value }))}
                        placeholder="mis. Proyek, Toko cabang"
                      />
                    </FormField>
                    <FormField label="Penerima" htmlFor="f-ship-recipient" errorText={fieldErrors.shipToRecipient}>
                      <TextInput
                        id="f-ship-recipient"
                        value={shipAdHoc.recipient}
                        maxLength={150}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, recipient: e.target.value }))}
                      />
                    </FormField>
                    <FormField label="Telepon penerima" htmlFor="f-ship-phone" errorText={fieldErrors.shipToPhone}>
                      <TextInput
                        id="f-ship-phone"
                        value={shipAdHoc.phone}
                        maxLength={30}
                        onChange={(e) => setShipAdHoc((a) => ({ ...a, phone: e.target.value }))}
                      />
                    </FormField>
                    <div className="sm:col-span-2">
                      <FormField
                        label="Alamat kirim"
                        htmlFor="f-ship-address"
                        helperText="Kosongkan semua untuk memakai alamat pelanggan sendiri. Disalin ke surat jalan ini saja."
                        errorText={fieldErrors.shipToAddress}
                      >
                        <TextArea
                          id="f-ship-address"
                          rows={2}
                          value={shipAdHoc.address}
                          maxLength={1000}
                          onChange={(e) => setShipAdHoc((a) => ({ ...a, address: e.target.value }))}
                          placeholder="Alamat lengkap"
                        />
                      </FormField>
                    </div>
                  </div>
                )}
                <div className="sm:col-span-1 lg:col-span-2">
                  <FormField label="Catatan turun barang" errorText={fieldErrors.dropLocationNote}>
                    <TextArea
                      rows={2}
                      value={dropLocationNote}
                      onChange={(e) => setDropLocationNote(e.target.value)}
                      placeholder="Titik/lokasi turun barang (opsional)"
                    />
                  </FormField>
                </div>
                <div className="sm:col-span-1 lg:col-span-2">
                  <FormField label="Catatan" errorText={fieldErrors.notes}>
                    <TextArea
                      rows={2}
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder="Keterangan tambahan (opsional)"
                    />
                  </FormField>
                </div>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </SectionCard>

        <SectionCard
          title="Barang"
          description="Hanya barang berstok. Satuan diambil dari master produk; lokasi asal boleh dikosongkan (server mengalokasikan stok lintas lokasi)."
        >
          <div className="flex flex-col gap-4">
            <FormField label="Tambah produk" htmlFor="f-product" errorText={fieldErrors.items}>
              <ProductSearchSelect
                id="f-product"
                value={null}
                trackedOnly
                preferStock
                placeholder="Cari produk berstok (kode atau nama)…"
                onChange={(_v, product) => {
                  if (product) addLine(product);
                }}
              />
            </FormField>

            <LineItemsEditor<DirectLineRow>
              lines={lines}
              getKey={(r) => r.key}
              onRemove={removeLine}
              emptyText="Belum ada barang. Cari produk di atas untuk menambahkan baris."
              columns={[
                {
                  id: "product",
                  header: "Produk",
                  render: (r) => (
                    <div className="min-w-44">
                      <p className="font-medium">{r.productLabel}</p>
                      {r.variants.length <= 1 ? (
                        <p className="text-xs text-muted-foreground">
                          {r.variants.find((v) => v.id === r.variantId)?.name ?? ""}
                        </p>
                      ) : (
                        <div className="mt-1">
                          <SelectInput
                            value={String(r.variantId)}
                            onChange={(e) => patchLine(r.key, { variantId: Number(e.target.value) })}
                            aria-label={`Varian ${r.productLabel}`}
                          >
                            {r.variants.map((v) => (
                              <option key={v.id} value={String(v.id)}>
                                {v.code} — {v.name}
                              </option>
                            ))}
                          </SelectInput>
                        </div>
                      )}
                    </div>
                  ),
                },
                {
                  id: "uom",
                  header: "Satuan",
                  width: "7rem",
                  render: (r) => (
                    <SelectInput
                      value={r.uom}
                      onChange={(e) => patchLine(r.key, { uom: e.target.value })}
                      aria-label={`Satuan ${r.productLabel}`}
                    >
                      {r.uoms.map((u) => (
                        <option key={u} value={u}>
                          {u}
                        </option>
                      ))}
                    </SelectInput>
                  ),
                },
                {
                  id: "qty",
                  header: "Qty kirim",
                  width: "8rem",
                  align: "right",
                  render: (r) => (
                    <TextInput
                      type="number"
                      min="0"
                      step="any"
                      value={r.qty}
                      onChange={(e) => patchLine(r.key, { qty: e.target.value })}
                      aria-label={`Qty kirim ${r.productLabel}`}
                      className="text-right"
                    />
                  ),
                },
                {
                  id: "location",
                  header: "Lokasi asal (opsional)",
                  width: "16rem",
                  render: (r) => (
                    <SearchSelect
                      aria-label={`Lokasi asal ${r.productLabel}`}
                      options={(locations ?? []).map((l) => ({ value: l.id, label: `${l.code} — ${l.name}` }))}
                      value={r.locationId}
                      placeholder="Otomatis (server)"
                      allowClear
                      onChange={(v) => patchLine(r.key, { locationId: toId(v) })}
                    />
                  ),
                },
              ]}
            />
          </div>
        </SectionCard>

        <StickyFooter
          summary={
            <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span>
                <span className="tabular">{lines.length}</span> baris
              </span>
              <span>
                Total qty <Qty value={totalQty} className="font-semibold text-foreground" />
              </span>
              {errorCount > 0 && (
                <span role="alert" className="font-medium text-destructive">
                  {errorCount} isian perlu diperbaiki
                </span>
              )}
            </span>
          }
        >
          <Button variant="secondary" asChild>
            <Link to={ROUTE_PATHS.deliveries}>Batal</Link>
          </Button>
          <Button type="submit" loading={saving}>
            Simpan surat jalan
          </Button>
        </StickyFooter>
      </form>
    </Page>
  );
}
