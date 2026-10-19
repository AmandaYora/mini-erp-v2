// Surat Transfer Barang — A4 landscape (ST-02), pola yang sama dengan Surat Jalan.
//
// Data: stockService.getTransfer + nama cabang, lokasi kedua cabang, dan produk
// (best-effort). Tanda tangan tiga pihak: Gudang Asal, Sopir, Gudang Tujuan.

import { useParams, useSearchParams } from "react-router-dom";
import { formatDate, formatDateTime, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { Letterhead } from "@/modules/print/components/Letterhead";
import { PrintProfileNotice } from "@/modules/print/components/PrintProfileNotice";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import { readPaperMode, withPaperModeParam, type PrintPaperMode } from "@/modules/print/paper";
import { continuousDeliveryStyles, deliveryStyles } from "@/modules/print/styles";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockService } from "@/modules/stock/services/stock.service";
import type { BranchOption, ProductDetail, StockLocation, StockTransfer } from "@/modules/stock/types";

function textOrDash(v: string | null | undefined): string {
  return v !== null && v !== undefined && v.trim() !== "" ? v : "-";
}

interface TransferPrintData {
  transfer: StockTransfer;
  branches: BranchOption[];
  fromLocations: StockLocation[];
  toLocations: StockLocation[];
  details: Map<number, ProductDetail>;
}

async function loadTransfer(id: number): Promise<TransferPrintData> {
  const transfer = await stockService.getTransfer(id);
  const [branches, fromLocations, toLocations] = await Promise.all([
    stockService.listBranches().catch(() => [] as BranchOption[]),
    stockService.listLocations(undefined, transfer.fromBranchId).catch(() => [] as StockLocation[]),
    stockService.listLocations(undefined, transfer.toBranchId).catch(() => [] as StockLocation[]),
  ]);
  const ids = [...new Set(transfer.items.map((it) => it.productId))];
  const fetched = await Promise.all(ids.map((pid) => stockService.getProduct(pid).catch(() => null)));
  const details = new Map<number, ProductDetail>();
  for (const d of fetched) if (d) details.set(d.id, d);
  return { transfer, branches, fromLocations, toLocations, details };
}

export default function StockTransferPrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, paper, corrupt, warning, loading: profileLoading } = usePrintProfile();
  const transferId = Number(id);

  const { data, loading, error } = useAsyncData(async (): Promise<TransferPrintData> => {
    if (!Number.isInteger(transferId) || transferId <= 0) throw new Error("ID transfer tidak valid.");
    try {
      return await loadTransfer(transferId);
    } catch (err) {
      toast.danger("Gagal memuat transfer", toApiError(err).message);
      throw err;
    }
  }, [transferId]);

  if (loading || profileLoading) return <div style={{ padding: "2rem" }}>Memuat surat transfer…</div>;
  if (error !== null || data === null) {
    return <div style={{ padding: "2rem" }}>{error ?? "Transfer tidak ditemukan."}</div>;
  }

  const { transfer, branches, fromLocations, toLocations, details } = data;
  const paperMode: PrintPaperMode = readPaperMode(searchParams, paper.isDefault ? "continuous" : "a4");
  const isContinuous = paperMode === "continuous";
  const fillerRows = isContinuous ? 0 : Math.max(0, 5 - transfer.items.length);

  const branchName = (bid: number) => {
    const b = branches.find((x) => x.id === bid);
    return b ? `${b.code} — ${b.name}` : `#${bid}`;
  };
  const locName = (list: StockLocation[], lid: number) => {
    const l = list.find((x) => x.id === lid);
    return l ? `${l.code} — ${l.name}` : `#${lid}`;
  };
  const statusText =
    transfer.status === "received"
      ? "Diterima"
      : transfer.status === "dispatched"
        ? "Dalam perjalanan"
        : transfer.status === "cancelled"
          ? "Dibatalkan"
          : "Draf";

  return (
    <PrintLayout
      baseCss={deliveryStyles()}
      continuousCss={isContinuous ? continuousDeliveryStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={null}
      notice={<PrintProfileNotice corrupt={corrupt} warning={warning} />}
      mainClassName={isContinuous ? "sj-page continuous" : "sj-page"}
    >
      <div className="sj-head">
        <Letterhead identity={identity} />
        <div>
          <div className="sj-title">Surat Transfer Barang</div>
          <div className="sj-meta">{transfer.number}</div>
        </div>
      </div>

      <div className="sj-parties">
        <div className="sj-kv">
          <span className="lbl">Dari Cabang</span><span>:</span><span>{branchName(transfer.fromBranchId)}</span>
          <span className="lbl">Gudang Asal</span><span>:</span><span>{locName(fromLocations, transfer.fromLocationId)}</span>
          <span className="lbl">Ke Cabang</span><span>:</span><span>{branchName(transfer.toBranchId)}</span>
          <span className="lbl">Gudang Tujuan</span><span>:</span><span>{locName(toLocations, transfer.toLocationId)}</span>
        </div>
        <div className="sj-kv">
          <span className="lbl">Tanggal Transfer</span>
          <span>:</span>
          <span>{transfer.transferDate ? formatDate(transfer.transferDate) : "-"}</span>
          <span className="lbl">Sopir</span><span>:</span><span>{textOrDash(transfer.driverName)}</span>
          <span className="lbl">Kendaraan</span><span>:</span><span>{textOrDash(transfer.vehiclePlate)}</span>
          <span className="lbl">Status</span><span>:</span><span>{statusText}</span>
        </div>
      </div>

      <table className="doc-table sj-table">
        <colgroup>
          <col style={{ width: "5%" }} />
          <col style={{ width: "12%" }} />
          <col />
          <col style={{ width: "28%" }} />
        </colgroup>
        <thead>
          <tr>
            <th>No</th>
            <th>Qty</th>
            <th>Produk</th>
            <th>Keterangan</th>
          </tr>
        </thead>
        <tbody>
          {transfer.items.map((item, idx) => {
            const d = details.get(item.productId);
            const v = d?.variants.find((x) => x.id === item.variantId);
            return (
              <tr key={`${item.productId}-${item.variantId}-${idx}`}>
                <td className="t-c">{idx + 1}</td>
                <td className="t-r">
                  {formatNumber(item.qty)}
                  {d?.baseUom ? ` ${d.baseUom}` : ""}
                </td>
                <td>
                  {d ? (
                    <>
                      <span style={{ fontWeight: 600 }}>{d.name}</span>{" "}
                      <span style={{ fontSize: "0.85em", color: "#4b5563" }}>
                        {d.code}
                        {v ? ` / ${v.code}` : ""}
                      </span>
                    </>
                  ) : (
                    <span style={{ color: "#6b7280" }}>
                      #{item.productId}/{item.variantId}
                    </span>
                  )}
                </td>
                <td>{item.notes}</td>
              </tr>
            );
          })}
          {Array.from({ length: fillerRows }).map((_, index) => (
            <tr key={`filler-${index}`}>
              <td>&nbsp;</td>
              <td />
              <td />
              <td />
            </tr>
          ))}
        </tbody>
      </table>

      {transfer.notes && transfer.notes.trim() !== "" ? <div className="sj-note">Catatan: {transfer.notes}</div> : null}

      <div className="sj-foot">
        <div />
        <div>
          <div className="sj-cityline">
            {identity.cityLine !== "" ? `${identity.cityLine}, ` : ""}................
          </div>
          <div className="sj-sign-grid">
            <div className="sj-sign-box">
              <div className="sj-sign-role">Gudang Asal</div>
              <div className="sj-sign-line">&nbsp;</div>
            </div>
            <div className="sj-sign-box">
              <div className="sj-sign-role">Sopir</div>
              <div className="sj-sign-line">{textOrDash(transfer.driverName)}</div>
            </div>
            <div className="sj-sign-box">
              <div className="sj-sign-role">Gudang Tujuan</div>
              <div className="sj-sign-line">&nbsp;</div>
            </div>
          </div>
        </div>
      </div>

      <div className="sj-footbar">
        <span>Dicetak: {formatDateTime(new Date().toISOString())}</span>
        <span>USER : {user?.fullName?.trim() !== "" ? user?.fullName : "-"}</span>
      </div>
    </PrintLayout>
  );
}
