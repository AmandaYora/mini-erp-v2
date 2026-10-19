package transform

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// ---------------------------------------------------------------- permissions

// rolePermissions maps the legacy permission codes of each migrated role to the
// revamp codes with the bootstrap legacy map (F3b Lampiran A), the single
// source of truth also used by the role seed. Codes the map does not know are
// reported (never silently dropped); codes dropped by design are info.
func (s *state) rolePermissions(roleSkip map[int64]bool) {
	permCode := map[int64]string{}
	for _, r := range s.raw["permissions"] {
		permCode[r.Int("id_permission")] = r.Str("permission_code")
	}
	byRole := map[int64][]string{}
	for _, r := range s.raw["role_permissions"] {
		if roleSkip[r.Int("id_role")] {
			continue
		}
		if code := permCode[r.Int("id_permission")]; code != "" {
			byRole[r.Int("id_role")] = append(byRole[r.Int("id_role")], code)
		}
	}
	seenCode := map[string]bool{}
	for _, r := range sortedBy(s.raw["roles"], "id_role") {
		lid := r.Int("id_role")
		ro, ok := s.id("roles", lid)
		if !ok {
			continue
		}
		codes := byRole[lid]
		sort.Strings(codes)
		for _, c := range codes {
			seenCode[c] = true
		}
		for _, t := range bootstrap.MapLegacyCodes(codes) {
			s.emit("role_permissions", row.Row{"role_id": ro, "permission_code": t})
		}
	}
	var all []string
	for c := range seenCode {
		all = append(all, c)
	}
	sort.Strings(all)
	dropped := map[string]bool{}
	for _, c := range bootstrap.DroppedLegacyCodes() {
		dropped[c] = true
	}
	unknown := map[string]bool{}
	for _, c := range bootstrap.UnmappedLegacyCodes(all) {
		unknown[c] = true
	}
	for _, c := range all {
		switch {
		case unknown[c]:
			s.find(report.Warn, "PERM_UNKNOWN", "permissions", 0, "kode izin legacy %q tidak ada di peta bootstrap/legacymap: tidak dimigrasi, putuskan padanannya", c)
		case dropped[c]:
			s.find(report.Info, "PERM_DROPPED", "permissions", 0, "kode izin legacy %q dibuang by design", c)
		}
	}
	s.out.Explain["permission_unknown_codes"] = int64(len(unknown))
}

// ---------------------------------------------------------------- company settings

func jsonObject(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func firstString(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if str, ok := v.(string); ok {
				return str, true
			}
		}
	}
	return "", false
}

// settings maps the five legacy JSON blobs to the registered key/value
// settings of the company module. What legacy always enforced is written
// explicitly (every stock adjustment needs an approver; delivery confirmation
// needs paperwork); the rest of the blobs is archived whole.
func (s *state) settings() {
	for _, r := range s.raw["company_settings"] {
		lid := r.Int(coKey(r))
		upd := tsOr(r, "updated_at", s.epoch())
		rules := jsonObject(r.Str("operational_rules_json"))
		ui := jsonObject(r.Str("ui_preferences_json"))
		kv := map[string]string{
			"stock_adjust_always_approve": "true", // legacy: no adjustment without approval (stock.service verifyStockAdjustmentApproval)
			"delivery_require_evidence":   "true", // legacy: confirmation needs signature status + document photo
		}
		mode := "simple"
		if v, ok := firstString(rules, "stockAdjustmentApprovalMode", "stock_adjustment_approval_mode"); ok && v == "strict" {
			mode = "strict"
		}
		kv["stock_adjust_approval_mode"] = mode
		for _, k := range []string{"stockAdjustmentApprovalThreshold", "stock_adjustment_approval_threshold"} {
			if f, ok := rules[k].(float64); ok && f >= 0 {
				kv["stock_adjust_approval_threshold"] = strconv.FormatFloat(f, 'f', -1, 64)
			}
		}
		if dp, ok := ui["documentProfile"].(map[string]any); ok && len(dp) > 0 {
			if raw, err := json.Marshal(dp); err == nil {
				kv["document_profile"] = string(raw)
			}
		}
		keys := make([]string, 0, len(kv))
		for k := range kv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s.emit("company_settings", row.Row{"setting_key": k, "setting_value": kv[k], "updated_at": upd})
		}
		s.find(report.Info, "SETTINGS_ARCHIVED", "company_settings", lid, "business_labels/assistant/reporting dan sisa ui/operational preferences diarsipkan utuh")
		return // single company
	}
}

// ---------------------------------------------------------------- audit logs

var auditEntity = map[string]struct{ revamp, table string }{
	"sales_return": {"sales_return", "sales_returns"}, "purchase_return": {"purchase_return", "purchase_returns"},
	"payment": {"payment", "payments"}, "delivery_note": {"delivery", "delivery_notes"}, "goods_receipt": {"goods_receipt", "goods_receipts"},
	"stock_transfer": {"stock_transfer", "stock_transfers"}, "product": {"product", "products"}, "user": {"user", "users"}, "role": {"role", "roles"},
	"branch": {"branch", "branches"}, "member_type": {"member_type", "member_types"}, "product_category": {"category", "product_categories"},
	"stock_location": {"stock_location", "stock_locations"}, "finance_account": {"account", "finance_accounts"},
	"business_expense": {"expense", "business_expenses"},
}

// auditLogs loads the audit trail of the current year (journal dates >= the
// opening date); older rows stay in the archive. Entity ids are translated
// through the id map; unknown entity kinds keep their name with a NULL id.
func (s *state) auditLogs() {
	partyType := map[int64]string{}
	for _, p := range s.raw["business_parties"] {
		partyType[p.Int("id_business_party")] = p.Str("party_type")
	}
	unmappedType := map[string]int64{}
	missingID := map[string]int64{}
	for _, a := range sortedBy(s.raw["audit_logs"], "id_audit_log") {
		when, ok := a.Time("happened_at")
		if !ok || wibDate(when) < s.open {
			s.out.Explain["audit_prior"]++
			continue
		}
		lid := a.Int("id_audit_log")
		etype := a.Str("entity_type")
		var entityID any
		outType := etype
		eid, hasID := a.NInt("id_entity")
		switch {
		case etype == "order" && hasID:
			if k := s.orderKind[eid]; k != "" {
				outType = k + "_order"
				if id, ok := s.id(k+"_orders", eid); ok {
					entityID = id
				}
			} else {
				missingID[etype]++
			}
		case etype == "business_party" && hasID:
			outType = "customer"
			if partyType[eid] == "supplier" {
				outType = "supplier"
			}
			if id, ok := s.id("parties", eid); ok {
				entityID = id
			} else {
				missingID[etype]++
			}
		default:
			m, known := auditEntity[etype]
			if !known {
				unmappedType[etype]++
				break
			}
			outType = m.revamp
			if hasID {
				if id, ok := s.id(m.table, eid); ok {
					entityID = id
				} else {
					missingID[etype]++
				}
			}
		}
		var branch any
		if b, ok := a.NInt("id_branch"); ok {
			if id, ok := s.id("branches", b); ok {
				branch = id
			}
		}
		var actor any
		if u, ok := a.NInt("id_actor"); ok {
			actor = s.userID(u)
		}
		note := any(nil)
		if after := strings.TrimSpace(a.Str("after_json")); after != "" {
			compact := after
			var v any
			if json.Unmarshal([]byte(after), &v) == nil {
				if raw, err := json.Marshal(v); err == nil {
					compact = string(raw)
				}
			}
			if utf8.RuneCountInString(compact) > 255 {
				compact = string([]rune(compact)[:255])
				s.out.Explain["audit_notes_truncated"]++
			}
			note = compact
		}
		atype := a.Str("actor_type")
		if utf8.RuneCountInString(atype) > 20 {
			atype = string([]rune(atype)[:20])
		}
		if atype == "" {
			atype = "user"
		}
		action := a.Str("action_key")
		if utf8.RuneCountInString(action) > 100 {
			action = string([]rune(action)[:100])
		}
		if len(outType) > 50 {
			outType = outType[:50]
		}
		s.emit("audit_logs", row.Row{
			"id": s.nextID("audit_logs"), "action_key": action, "entity_type": outType, "entity_id": entityID, "branch_id": branch,
			"actor_type": atype, "actor_id": actor, "note": note, "created_at": fmtUTC(when),
		})
		_ = lid
	}
	for _, t := range sortedKeys64(unmappedType) {
		s.find(report.Info, "AUD_ENTITY_TYPE", "audit_logs", 0, "%d baris jenis entitas %q tanpa padanan: id entitas dikosongkan", unmappedType[t], t)
	}
	for _, t := range sortedKeys64(missingID) {
		s.find(report.Info, "AUD_ENTITY_ID", "audit_logs", 0, "%d baris jenis %q menunjuk entitas yang tidak termigrasi: id dikosongkan", missingID[t], t)
	}
}

func sortedKeys64(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
