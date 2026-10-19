// Validasi form "Pelanggan baru" di POS (PR-02) — murni agar bisa diuji.
// Aturan telepon disalin dari backend validPhone: digit 7–20, selain digit
// hanya + spasi - ( ). Bayar Nanti = termin net, yang mewajibkan telepon.

export interface NewCustomerInput {
  name: string;
  phone: string;
  memberTypeId: number | null;
  requirePhone: boolean;
}

export interface NewCustomerErrors {
  name?: string;
  phone?: string;
}

function phoneOk(v: string): boolean {
  let digits = 0;
  for (const ch of v) {
    if (ch >= "0" && ch <= "9") digits++;
    else if (!"+ -()".includes(ch)) return false;
  }
  return digits >= 7 && digits <= 20;
}

export function validateNewCustomer(i: NewCustomerInput): NewCustomerErrors {
  const errors: NewCustomerErrors = {};
  const name = i.name.trim();
  const phone = i.phone.trim();
  if (name === "") errors.name = "Nama wajib diisi";
  else if (name.length > 200) errors.name = "Nama maksimal 200 karakter";
  if (phone === "") {
    if (i.requirePhone) errors.phone = "Bayar Nanti (termin net) wajib memakai pelanggan ber-telepon";
  } else if (!phoneOk(phone)) {
    errors.phone = "Format telepon tidak valid (7–20 digit)";
  }
  return errors;
}

/** Body POST /customers dari isian yang sudah valid. */
export function newCustomerBody(i: NewCustomerInput): { name: string; phone?: string; memberTypeId?: number } {
  const phone = i.phone.trim();
  return {
    name: i.name.trim(),
    ...(phone !== "" ? { phone } : {}),
    ...(i.memberTypeId !== null ? { memberTypeId: i.memberTypeId } : {}),
  };
}
