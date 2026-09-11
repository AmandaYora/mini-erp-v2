// Faktur / Nota Penjualan — A4 landscape + kertas kontinu (Tahap E3).
//
// Data: salesService.get (orderView sudah membawa ship-to C2 + nomor faktur
// pajak C3). Uang integer rupiah; terbilang dari modul ini.

import { useParams, useSearchParams } from "react-router-dom";
import { formatDate, formatDateTime, formatIDR, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { PrintLayout } from "@/modules/print/components/PrintLayout";
import { BankAccountsBlock, Letterhead } from "@/modules/print/components/Letterhead";
import { resolveShipToContact, terbilangRupiah } from "@/modules/print/document";
import {
  readHideLetterhead,
  readPaperMode,
  withHideLetterheadParam,
  withPaperModeParam,
  type PrintPaperMode,
} from "@/modules/print/paper";
import { usePrintProfile } from "@/modules/print/hooks/usePrintProfile";
import { fetchSalesOrderForPrint, type SalesOrder } from "@/modules/print/services/print-fetch";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { continuousInvoiceStyles, invoiceStyles } from "@/modules/print/styles";

export default function SalesInvoicePrintPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const { identity, paper, corrupt, loading: profileLoading } = usePrintProfile();

  const orderId = Number(id);

  const { data: order, loading, error } = useAsyncData(
    async (): Promise<SalesOrder> => {
      if (!Number.isInteger(orderId) || orderId <= 0) {
        throw new Error("ID order tidak valid.");
      }
      try {
        return await fetchSalesOrderForPrint(orderId);
      } catch (err) {
        const msg = toApiError(err).message;
        toast.danger("Gagal memuat faktur", msg);
        throw err;
      }
    },
    [orderId],
  );

  if (loading || profileLoading) {
    return <div style={{ padding: "2rem" }}>Memuat faktur…</div>;
  }
  if (error !== null || order === null) {
    return (
      <div style={{ padding: "2rem" }}>{error ?? "Order tidak ditemukan."}</div>
    );
  }

  const paperMode: PrintPaperMode = readPaperMode(
    searchParams,
    paper.isDefault ? "continuous" : "a4",
  );
  const isContinuous = paperMode === "continuous";
  const fillerRows = isContinuous ? 0 : Math.max(0, 6 - order.items.length);
  const hideLetterhead = readHideLetterhead(searchParams, !paper.showLetterhead);
  const showStatic = !isContinuous || !hideLetterhead;

  const shipToRecipient = order.shipToRecipient.trim();
  const contact = resolveShipToContact({
    shipToAddress: order.shipToAddress,
    shipToPhone: order.shipToPhone,
    fallbackAddress: null,
    fallbackPhone: null,
  });
  const customerLine = order.partyName.trim() !== "" ? order.partyName : "-";

  return (
    <PrintLayout
      baseCss={invoiceStyles()}
      continuousCss={isContinuous ? continuousInvoiceStyles(paper) : null}
      paperMode={paperMode}
      onPaperModeChange={(m) => setSearchParams(withPaperModeParam(searchParams, m), { replace: true })}
      letterheadToggle={{
        hidden: hideLetterhead,
        onChange: (hidden) =>
          setSearchParams(withHideLetterheadParam(searchParams, hidden), { replace: true }),
      }}
      notice={
        corrupt ? (
          <div className="no-print print-warning">
            Profil kertas tersimpan rusak — memakai default 241,3 × 139,7 mm. Perbaiki di kalibrasi kertas.
          </div>
        ) : undefined
      }
      mainClassName={isContinuous ? "nota-page continuous" : "nota-page"}
    >
      <div className="nota-head">
        {showStatic ? <Letterhead identity={identity} /> : <div />}
        <div>
          <div className="nota-title">Faktur</div>
          <div className="nota-meta">{order.number}</div>
        </div>
      </div>

      <div className="nota-parties">
        <div className="nota-kv">
          <span className="lbl">Pelanggan</span><span>:</span><span>{customerLine}</span>
          {shipToRecipient !== "" && shipToRecipient !== customerLine ? (
            <>
              <span className="lbl">Penerima</span><span>:</span><span>{shipToRecipient}</span>
            </>
          ) : null}
          {order.shipToLabel.trim() !== "" ? (
            <>
              <span className="lbl">Label Kirim</span><span>:</span><span>{order.shipToLabel}</span>
            </>
          ) : null}
          <span className="lbl">Alamat</span><span>:</span><span>{contact.address}</span>
          <span className="lbl">No HP</span><span>:</span><span>{contact.phone}</span>
        </div>
        <div className="nota-kv">
          <span className="lbl">Tanggal</span><span>:</span><span>{formatDate(order.orderDate)}</span>
          <span className="lbl">Jatuh Tempo</span><span>:</span><span>{formatDate(order.dueDate)}</span>
          <span className="lbl">Termin</span><span>:</span><span>{order.paymentTerms === "net" ? "Tempo" : "Tunai"}</span>
          {order.taxInvoiceNumber.trim() !== "" ? (
            <>
              <span className="lbl">Faktur Pajak</span><span>:</span><span>{order.taxInvoiceNumber}</span>
              <span className="lbl">Tgl Pajak</span><span>:</span><span>{formatDate(order.taxInvoiceDate)}</span>
            </>
          ) : null}
        </div>
      </div>

      <table className="doc-table nota-table">
        <colgroup>
          <col style={{ width: "4%" }} />
          <col style={{ width: "10%" }} />
          <col />
          <col style={{ width: "8%" }} />
          <col style={{ width: "7%" }} />
          <col style={{ width: "13%" }} />
          <col style={{ width: "10%" }} />
          <col style={{ width: "14%" }} />
        </colgroup>
        <thead>
          <tr>
            <th>No</th>
            <th>Kode</th>
            <th>Produk</th>
            <th>Qty</th>
            <th>Satuan</th>
            <th>Harga</th>
            <th>Diskon</th>
            <th>Jumlah</th>
          </tr>
        </thead>
        <tbody>
          {order.items.map((item, idx) => {
            const discount =
              item.discountNominal > 0
                ? formatIDR(item.discountNominal)
                : item.discountPct > 0
                  ? `${formatNumber(item.discountPct)}%`
                  : "-";
            return (
              <tr key={item.id}>
                <td className="t-c">{idx + 1}</td>
                <td>{item.productCode}</td>
                <td>{item.productName}</td>
                <td className="t-r">{formatNumber(item.qty)}</td>
                <td className="t-c">{item.uom}</td>
                <td className="t-r">{formatIDR(item.unitPrice)}</td>
                <td className="t-c">{discount}</td>
                <td className="t-r">{formatIDR(item.lineTotal)}</td>
              </tr>
            );
          })}
          {Array.from({ length: fillerRows }).map((_, index) => (
            <tr key={`filler-${index}`}>
              <td>&nbsp;</td>
              <td />
              <td />
              <td />
              <td />
              <td />
              <td />
              <td />
            </tr>
          ))}
        </tbody>
      </table>

      <div className="nota-sumbar">
        <div className="nota-terbilang">
          Terbilang: {terbilangRupiah(order.grandTotal)}
        </div>
        <table className="nota-paysum">
          <tbody>
            <tr>
              <td>Subtotal</td>
              <td>{formatIDR(order.subtotal)}</td>
            </tr>
            {order.discountTotal > 0 ? (
              <tr>
                <td>Diskon</td>
                <td>{formatIDR(order.discountTotal)}</td>
              </tr>
            ) : null}
            {order.taxTotal > 0 ? (
              <tr>
                <td>PPN</td>
                <td>{formatIDR(order.taxTotal)}</td>
              </tr>
            ) : null}
            <tr className="bold">
              <td>Total</td>
              <td>{formatIDR(order.grandTotal)}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="nota-foot">
        <div className="nota-buyer">
          <div>Pembeli</div>
          <div className="nota-buyer-line">( .................... )</div>
        </div>
        <div className="nota-sell">
          {showStatic && identity.sellingPoints.length > 0 ? (
            <>
              <div className="nota-sell-title">MENJUAL:</div>
              <ul>
                {identity.sellingPoints.map((point) => (
                  <li key={point}>{point}</li>
                ))}
              </ul>
            </>
          ) : null}
          {identity.returnNote !== "" ? (
            <div style={{ marginTop: "6px" }}>{identity.returnNote}</div>
          ) : null}
        </div>
        <div>
          {showStatic && identity.bankAccounts.length > 0 ? (
            <BankAccountsBlock accounts={identity.bankAccounts} />
          ) : null}
        </div>
      </div>

      <div className="nota-footbar">
        <span>Dicetak: {formatDateTime(new Date().toISOString())}</span>
        <span>
          {identity.cityLine !== "" ? `${identity.cityLine}, ` : ""}
          {formatDate(order.orderDate)}
        </span>
        <span>USER : {user?.fullName?.trim() !== "" ? user?.fullName : "-"}</span>
      </div>
    </PrintLayout>
  );
}
