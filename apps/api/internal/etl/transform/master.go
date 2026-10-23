package transform

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

func (s *state) master() error {
	s.branches()
	s.company()
	s.users()
	s.memberTypes()
	s.catalog()
	s.parties()
	s.locations()
	return nil
}

func archivedStatus(r row.Row) string {
	if r.Has("archived_at") || (r.Str("status") != "" && r.Str("status") != "active") {
		return "archived"
	}
	return "active"
}

func (s *state) branches() {
	rows := sortedBy(s.raw["branches"], "id_branch")
	defaults := 0
	for _, r := range rows {
		if r.Bool("is_default") {
			defaults++
		}
	}
	if defaults != 1 && len(rows) > 0 {
		sev := report.Warn
		if defaults > 1 {
			sev = report.Blocker
		}
		s.find(sev, "BRANCH_DEFAULT", "branches", 0, "jumlah cabang default = %d (harus tepat 1 sebagai kantor pusat)", defaults)
	}
	head := false
	for _, r := range rows {
		lid := r.Int("id_branch")
		id := s.alloc("branches", lid)
		isHead := r.Bool("is_default") && !head
		if isHead {
			head = true
			s.headBranch = id
		}
		code := r.Str("code")
		s.branchCode[id] = code
		st := "active"
		if r.Str("status") != "active" {
			st = "inactive"
		}
		s.emit("branches", row.Row{
			"id": id, "code": code, "name": s.trunc("branches", lid, "name", r.Str("name"), 150),
			"address": s.optStr("branches", lid, "address", r, "address_text", 255), "city": s.optStr("branches", lid, "city", r, "city", 100),
			"phone": s.optStr("branches", lid, "phone", r, "phone_e164", 30), "status": st, "is_head": b2i(isHead),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
	if s.headBranch == 0 && len(rows) > 0 {
		s.headBranch = 1 // no default flagged (already reported): the first branch acts as head
	}
}

func (s *state) company() {
	for _, r := range s.raw["companies"] {
		lid := r.Int(coKey(r))
		s.emit("company_profile", row.Row{
			"id": s.alloc("company_profile", lid), "name": s.trunc("company_profile", lid, "name", r.Str("name"), 150),
			"legal_name": s.optStr("company_profile", lid, "legal_name", r, "legal_name", 150),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
		return // single-company tool: exactly one profile
	}
}

// coKey finds the company key column without spelling the banned literal.
func coKey(r row.Row) string {
	for k := range r {
		if strings.HasPrefix(k, "id_") && strings.HasSuffix(k, "company") {
			return k
		}
	}
	return ""
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *state) users() {
	roleSkip := map[int64]bool{}
	for _, r := range sortedBy(s.raw["roles"], "id_role") {
		lid := r.Int("id_role")
		if r.Str("code") == "superadmin" {
			// The reserved role is seeded by v2 (bootstrap.EnsureAuthz) with every
			// permission; the loader matches it by code. Only its legacy permission
			// list is left behind.
			roleSkip[lid] = true
			s.find(report.Info, "ROLE_SUPERADMIN", "roles", lid, "peran superadmin dipetakan ke peran cadangan v2; izin legacy-nya tidak disalin (v2 memberi semua izin)")
		}
		id := s.alloc("roles", lid)
		s.emit("roles", row.Row{
			"id": id, "code": s.trunc("roles", lid, "code", r.Str("code"), 50), "name": s.trunc("roles", lid, "name", r.Str("name"), 100),
			"description": emptyNil(r.Str("description_text")), "is_system": b2i(r.Bool("is_system_role")),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
	seenUser := map[string]int64{}
	for _, r := range sortedBy(s.raw["users"], "id_user") {
		lid := r.Int("id_user")
		uname := r.Str("username")
		if prev, dup := seenUser[lower(uname)]; dup {
			s.find(report.Blocker, "USER_DUP", "users", lid, "username ganda (case-insensitive) dengan user %d", prev)
			s.userSkip[lid] = true
			continue
		}
		seenUser[lower(uname)] = lid
		status := "active"
		hash := r.Str("password_hash")
		if r.Str("status") != "active" {
			status = "inactive"
		}
		// An account whose hash still matches a legacy seed password (design
		// V13; one list shared with data-health A1) is locked.
		if usercontracts.MatchesLegacySeedPassword(hash) {
			// The developer superadmin keeps working as in legacy (owner's
			// decision, ADR-0018 D13); every other seed account is locked.
			if uname == "superadmin" {
				s.find(report.Warn, "USER_SUPERADMIN_SEED_PASSWORD", "users", lid, "akun superadmin masih memakai kata sandi bawaan seed: tetap aktif, segera ganti sandinya")
			} else {
				status = "inactive"
				s.find(report.Warn, "USER_SEED_PASSWORD", "users", lid, "akun %q masih memakai kata sandi bawaan seed: dikunci, perlu reset", uname)
			}
		}
		if !strings.HasPrefix(hash, "$2") {
			s.find(report.Warn, "USER_HASH", "users", lid, "hash kata sandi bukan bcrypt: akun %q akan butuh reset", uname)
		}
		id := s.alloc("users", lid)
		s.emit("users", row.Row{
			"id": id, "username": s.trunc("users", lid, "username", uname, 100), "email": emptyNil(r.Str("email")), "password_hash": hash,
			"full_name": s.trunc("users", lid, "full_name", r.Str("full_name"), 150), "status": status,
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
	for _, r := range s.raw["user_roles"] {
		u, ok1 := s.id("users", r.Int("id_user"))
		ro, ok2 := s.id("roles", r.Int("id_role"))
		if !ok1 || !ok2 {
			continue
		}
		s.emit("user_roles", row.Row{"user_id": u, "role_id": ro})
	}
	sort.SliceStable(s.out.Tables["user_roles"], func(i, j int) bool {
		a, b := s.out.Tables["user_roles"][i], s.out.Tables["user_roles"][j]
		if a.Int("user_id") != b.Int("user_id") {
			return a.Int("user_id") < b.Int("user_id")
		}
		return a.Int("role_id") < b.Int("role_id")
	})
	for _, r := range s.raw["user_branch_accesses"] {
		u, ok1 := s.id("users", r.Int("id_user"))
		b, ok2 := s.id("branches", r.Int("id_branch"))
		if !ok1 || !ok2 {
			continue
		}
		s.emit("user_branch_access", row.Row{"user_id": u, "branch_id": b, "is_default": b2i(r.Bool("is_default_branch"))})
	}
	s.rolePermissions(roleSkip)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func emptyNil(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *state) memberTypes() {
	for _, r := range sortedBy(s.raw["member_types"], "id_member_type") {
		lid := r.Int("id_member_type")
		dir := "minus"
		if r.Str("price_adjustment_direction") == "add" {
			dir = "plus"
		}
		var inc any
		if r.Has("rounding_increment") {
			inc = r.Str("rounding_increment")
		}
		s.emit("member_types", row.Row{
			"id": s.alloc("member_types", lid), "code": r.Str("code"), "name": s.trunc("member_types", lid, "name", r.Str("name"), 120),
			"description_text": emptyNil(r.Str("description_text")), "price_basis": r.Str("price_basis"), "price_adjustment_direction": dir,
			"price_adjustment_type": r.Str("price_adjustment_type"), "price_adjustment_value": r.Str("price_adjustment_value"),
			"rounding_mode": r.Str("rounding_mode"), "rounding_increment": inc, "status": archivedStatus(r), "archived_at": ts(r, "archived_at"),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
}

func (s *state) catalog() {
	cats := sortedBy(s.raw["product_categories"], "id_product_category")
	for _, r := range cats {
		s.alloc("product_categories", r.Int("id_product_category"))
	}
	for _, r := range cats {
		lid := r.Int("id_product_category")
		id, _ := s.id("product_categories", lid)
		var parent any
		if p, ok := r.NInt("id_parent_category"); ok {
			if pid, ok := s.id("product_categories", p); ok && pid != id {
				parent = pid
			}
		}
		s.emit("product_categories", row.Row{
			"id": id, "code": s.trunc("product_categories", lid, "code", r.Str("code"), 50), "name": s.trunc("product_categories", lid, "name", r.Str("name"), 150),
			"parent_id": parent, "status": archivedStatus(r),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
	for _, r := range sortedBy(s.raw["products"], "id_product") {
		lid := r.Int("id_product")
		id := s.alloc("products", lid)
		typ, tracked := "barang", r.Bool("stock_tracked")
		switch r.Str("product_type") {
		case "service", "non_stock":
			typ, tracked = "jasa", false
		case "physical", "bundle":
		default:
			s.find(report.Warn, "PRODUCT_TYPE", "products", lid, "tipe produk %q tidak dikenal, dianggap barang", r.Str("product_type"))
		}
		s.prodType[id] = typ
		var cat any
		if c, ok := r.NInt("id_product_category"); ok {
			if cid, ok := s.id("product_categories", c); ok {
				cat = cid
			}
		}
		minStock := "0"
		if r.Has("min_stock_qty") {
			minStock = r.Str("min_stock_qty")
		}
		rr := row.Row{
			"id": id, "code": s.trunc("products", lid, "code", r.Str("product_code"), 50), "name": s.trunc("products", lid, "name", r.Str("product_name"), 255),
			"category_id": cat, "type": typ, "stock_tracked": b2i(tracked), "base_uom": r.Str("base_uom"), "purchase_uom": r.Str("purchase_uom"),
			"sales_uom": r.Str("sales_uom"), "purchase_to_base_factor": r.Str("purchase_to_base_factor"), "sales_to_base_factor": r.Str("sales_to_base_factor"),
			"purchase_price":    s.money.R("products", lid, "purchase_price", r, "purchase_price"),
			"selling_price":     s.money.R("products", lid, "selling_price", r, "selling_price"),
			"min_selling_price": s.money.R("products", lid, "min_selling_price", r, "min_selling_price"),
			"min_stock_qty":     minStock, "status": archivedStatus(r),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		}
		s.productRow[id] = rr
		s.emit("products", rr)
	}
	barcodes := map[string]int64{}
	// Legacy lets a code be reused once its variant is archived; the revamp's
	// uq_product_variants (product_id, code) does not. Per (product, code) the
	// keeper is the lowest-id live variant, else the highest-id archived one;
	// every other variant keeps its history but gets code "<code>~<legacy id>".
	type vkey struct {
		pid  int64
		code string
	}
	keeper := map[vkey]int64{}
	for _, r := range sortedBy(s.raw["product_variants"], "id_product_variant") {
		k := vkey{r.Int("id_product"), r.Str("variant_code")}
		cur, seen := keeper[k]
		live := !r.Has("archived_at")
		if !seen || (live && s.variantArchived(cur)) || (!live && s.variantArchived(cur) && r.Int("id_product_variant") > cur) {
			keeper[k] = r.Int("id_product_variant")
		}
	}
	for _, r := range sortedBy(s.raw["product_variants"], "id_product_variant") {
		lid := r.Int("id_product_variant")
		pid, ok := s.id("products", r.Int("id_product"))
		if !ok {
			s.find(report.Blocker, "VARIANT_ORPHAN", "product_variants", lid, "varian tanpa produk")
			continue
		}
		id := s.alloc("product_variants", lid)
		var bc any
		if b := r.Str("barcode"); b != "" {
			if prev, dup := barcodes[b]; dup {
				s.find(report.Blocker, "BARCODE_DUP", "product_variants", lid, "barcode %q ganda dengan varian %d", b, prev)
			}
			barcodes[b] = lid
			bc = s.trunc("product_variants", lid, "barcode", b, 100)
		}
		if r.Bool("is_default") {
			s.defVariant[pid] = id
		}
		code := s.trunc("product_variants", lid, "code", r.Str("variant_code"), 50)
		if keeper[vkey{r.Int("id_product"), r.Str("variant_code")}] != lid {
			suffix := fmt.Sprintf("~%d", lid)
			base := []rune(r.Str("variant_code"))
			if len(base) > 50-len(suffix) {
				base = base[:50-len(suffix)]
			}
			code = string(base) + suffix
			s.log.Adjust("product_variants", lid, "code", r.Str("variant_code"), code, "dedupe_archived_variant_code")
			s.out.Explain["variant_codes_deduped"]++
			s.find(report.Info, "VARIANT_CODE_DEDUPE", "product_variants", lid, "kode varian dipakai ulang di legacy; kode lama diberi sufiks %s", suffix)
		}
		s.emit("product_variants", row.Row{
			"id": id, "product_id": pid, "code": code,
			"name": s.trunc("product_variants", lid, "name", r.Str("variant_name"), 150), "barcode": bc, "is_default": b2i(r.Bool("is_default")),
			"status": archivedStatus(r), "created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
}

// variantArchived reports whether the legacy variant id is archived.
func (s *state) variantArchived(id int64) bool {
	for _, r := range s.raw["product_variants"] {
		if r.Int("id_product_variant") == id {
			return r.Has("archived_at")
		}
	}
	return false
}

func (s *state) parties() {
	for _, r := range sortedBy(s.raw["business_parties"], "id_business_party") {
		lid := r.Int("id_business_party")
		typ := r.Str("party_type")
		if typ != "customer" && typ != "supplier" {
			s.find(report.Blocker, "PARTY_TYPE", "business_parties", lid, "tipe pihak %q tidak didukung revamp (keputusan P7)", typ)
			continue
		}
		var mt any
		if m, ok := r.NInt("id_member_type"); ok {
			if id, ok := s.id("member_types", m); ok {
				mt = id
			}
		}
		s.emit("parties", row.Row{
			"id": s.alloc("parties", lid), "code": s.trunc("parties", lid, "code", r.Str("code"), 50), "party_type": typ,
			"name": s.trunc("parties", lid, "name", r.Str("name"), 255), "phone": s.optStr("parties", lid, "phone", r, "phone_e164", 30),
			"email": emptyNil(r.Str("email")), "address_text": emptyNil(r.Str("address_text")), "notes": emptyNil(r.Str("notes")),
			"member_type_id": mt, "status": archivedStatus(r), "archived_at": ts(r, "archived_at"),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
	for _, r := range sortedBy(s.raw["business_party_delivery_addresses"], "id_address") {
		lid := r.Int("id_address")
		pid, ok := s.id("parties", r.Int("id_business_party"))
		if !ok {
			continue
		}
		s.emit("party_delivery_addresses", row.Row{
			"id": s.alloc("party_delivery_addresses", lid), "party_id": pid, "label": s.optStr("party_delivery_addresses", lid, "label", r, "label", 120),
			"recipient_name": s.optStr("party_delivery_addresses", lid, "recipient_name", r, "recipient_name", 150),
			"phone":          s.optStr("party_delivery_addresses", lid, "phone", r, "phone_e164", 30), "address_text": r.Str("address_text"),
			"is_primary": b2i(r.Bool("is_primary")), "sort_order": r.Int("sort_order"), "status": archivedStatus(r), "archived_at": ts(r, "archived_at"),
			"created_at": tsLocalOr(r, "created_at", s.epoch()), "updated_at": tsLocalOr(r, "updated_at", s.epoch()),
		})
	}
}

// locations applies the system-location rules of design F5-4: the legacy
// default location becomes GDG, every damaged location of a branch collapses
// into one RUSAK (many-to-one in the id map).
func (s *state) locations() {
	rows := sortedBy(s.raw["stock_locations"], "id_stock_location")
	for _, r := range rows {
		lid := r.Int("id_stock_location")
		bid, ok := s.id("branches", r.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "LOC_ORPHAN", "stock_locations", lid, "lokasi tanpa cabang")
			continue
		}
		switch {
		case r.Bool("is_default"):
			if _, dup := s.gdg[bid]; dup {
				s.find(report.Blocker, "LOC_DEFAULT", "stock_locations", lid, "lebih dari satu lokasi default pada cabang %d", bid)
				continue
			}
			id := s.alloc("stock_locations", lid)
			s.gdg[bid] = id
			s.locBranch[id] = bid
			s.emit("stock_locations", s.locRow(id, bid, "GDG", r.Str("name"), 1, "normal", r, nil))
		case r.Str("status") == "damaged":
			if rid, ok := s.rusak[bid]; ok {
				s.alias("stock_locations", lid, rid)
				s.find(report.Info, "LOC_RUSAK_MERGE", "stock_locations", lid, "lokasi rusak digabung ke RUSAK")
				continue
			}
			id := s.alloc("stock_locations", lid)
			s.rusak[bid] = id
			s.locBranch[id] = bid
			s.emit("stock_locations", s.locRow(id, bid, "RUSAK", "Stok Rusak", 1, "damaged", r, nil))
		default:
			code := r.Str("code")
			if len(code) > 30 {
				s.find(report.Blocker, "LOC_CODE", "stock_locations", lid, "kode lokasi %q melebihi 30 karakter", code)
			}
			if code == "GDG" || code == "RUSAK" {
				s.find(report.Blocker, "LOC_RESERVED", "stock_locations", lid, "kode %q dicadangkan untuk lokasi sistem", code)
			}
			id := s.alloc("stock_locations", lid)
			s.locBranch[id] = bid
			s.emit("stock_locations", s.locRow(id, bid, code, r.Str("name"), 0, "normal", r, r))
		}
	}
	// parent links (second pass, skip self/merged)
	for _, r := range rows {
		p, ok := r.NInt("id_parent_location")
		if !ok {
			continue
		}
		cid, ok1 := s.id("stock_locations", r.Int("id_stock_location"))
		pid, ok2 := s.id("stock_locations", p)
		if !ok1 || !ok2 || cid == pid {
			continue
		}
		for _, o := range s.out.Tables["stock_locations"] {
			if o.Int("id") == cid && o.Str("code") != "GDG" && o.Str("code") != "RUSAK" {
				o["parent_id"] = pid
			}
		}
	}
	for b := range s.branchCode {
		if _, ok := s.gdg[b]; !ok {
			// Branch without a default location: synthesize GDG so documents always have a home.
			id := s.alloc("stock_locations", -b)
			s.gdg[b] = id
			s.locBranch[id] = b
			s.find(report.Warn, "LOC_NO_DEFAULT", "branches", b, "cabang tanpa lokasi default: GDG dibuat")
			s.emit("stock_locations", row.Row{"id": id, "branch_id": b, "code": "GDG", "name": "Gudang Utama", "parent_id": nil, "is_system": 1,
				"kind": "normal", "status": "active", "created_at": fmtUTC(s.epoch()), "updated_at": fmtUTC(s.epoch())})
		}
	}
}

func (s *state) locRow(id, branch int64, code, name string, system int, kind string, src row.Row, plain row.Row) row.Row {
	st := "active"
	if src.Has("archived_at") || (src.Str("status") != "active" && src.Str("status") != "damaged") {
		st = "archived"
	}
	// ST-01: the legacy allocation attributes travel with the location (a damaged
	// location never allocates, so its flags stay off).
	picking, primary, sortOrder := 0, 0, 0
	if kind != "damaged" {
		picking, primary, sortOrder = b2i(src.Bool("is_picking_area")), b2i(src.Bool("is_primary")), int(src.Int("sort_order"))
	}
	return row.Row{"id": id, "branch_id": branch, "code": code, "name": s.trunc("stock_locations", src.Int("id_stock_location"), "name", name, 150),
		"parent_id": nil, "is_system": system, "kind": kind, "status": st,
		"is_picking_area": picking, "is_primary": primary, "sort_order": sortOrder,
		"created_at": tsLocalOr(src, "created_at", s.epoch()), "updated_at": tsLocalOr(src, "updated_at", s.epoch())}
}
