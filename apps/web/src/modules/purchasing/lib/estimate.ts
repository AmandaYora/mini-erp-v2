// Estimasi total sisi klien. Replika persis purchasing/domain/pricing.go
// CalcLine (gross = round(qty×unit); disc = round(gross×pct/100);
// net = gross−disc−nominal; pajak per taxType). Hanya ESTIMASI — total
// resmi selalu dari server setelah simpan (harga & pajak di-snapshot
// server dari input + master produk).

export interface LineEstimate {
  gross: number;
  disc: number;
  net: number;
  tax: number;
  total: number;
}

export function calcLineEstimate(
  qty: number,
  unitPrice: number,
  discountPct: number,
  discountNominal: number,
  taxType: string,
  taxRate: number,
): LineEstimate {
  const gross = Math.round(qty * unitPrice);
  const disc = Math.round((gross * discountPct) / 100);
  const net = gross - disc - discountNominal;
  let tax = 0;
  let total = net;
  if (taxType === "exclude") {
    tax = Math.round((net * taxRate) / 100);
    total = net + tax;
  } else if (taxType === "include") {
    if (taxRate > 0) {
      const base = Math.round(net / (1 + taxRate / 100));
      tax = net - base;
    }
    total = net;
  }
  return { gross, disc, net, tax, total };
}
