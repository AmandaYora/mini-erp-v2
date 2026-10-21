package application

import (
	"context"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
	"mini-erp/internal/shared/validator"
)

// SJ Langsung (ADR-0022): a surat jalan drafted for a customer BEFORE any
// order exists. Until the sales module builds the order from it (LinkDirect),
// the note carries its own customer and ship-to snapshot, confirms with a
// stock-out only (no quota, no settlement, no order completion) and may be
// voided even after it shipped. Once linked it is an ordinary order note.
//
// Only stock-tracked goods travel on a direct note: a service or non-stock
// line belongs on the order that is built later (ADR-0022 K6), so a direct
// note posts every one of its lines.

// isUnorderedDirect reports a direct note that has no order yet.
func isUnorderedDirect(n *contracts.DeliveryNote) bool {
	return n.Origin == contracts.OriginDirect && n.SalesOrderID == 0
}

// noteLock is the lock key of a note that is not (yet) serialized by an order.
func noteLock(id int64) string { return "sj:" + strconv.FormatInt(id, 10) }

// DirectInput is the document face of a direct note: the customer, where the
// goods go, the crew, and the lines (no prices).
type DirectInput struct {
	PartyID      int64
	DeliveryDate string
	Notes        string
	// ShipToAddressID picks a saved customer address; else the ad-hoc fields
	// are snapshotted as typed (the address text is then required when any
	// detail is given). Neither = the customer's own address on paper.
	ShipToAddressID int64
	ShipToLabel     string
	ShipToRecipient string
	ShipToPhone     string
	ShipToAddress   string
	Doc             DocInput
	Lines           []LineInput
}

// Column widths of the ship-to snapshot (delivery_notes ship_to_*), the same
// as the order's.
const (
	maxShipToLabel     = 120
	maxShipToRecipient = 150
	maxShipToPhone     = 30
	maxShipToAddress   = 1000
)

// CreateDirect drafts a direct note. Idempotent per ClientRef like Create.
func (s *Service) CreateDirect(ctx context.Context, actorID, branchID int64, in DirectInput) (*contracts.DeliveryNote, bool, error) {
	in.Doc.ClientRef = strings.TrimSpace(in.Doc.ClientRef)
	if len(in.Doc.ClientRef) > 64 {
		return nil, false, apperror.Validation("", []apperror.FieldError{{Field: "clientRef", Message: "maksimal 64 karakter"}})
	}
	if in.Doc.ClientRef != "" {
		prior, err := s.directByClientRef(ctx, branchID, in.Doc.ClientRef)
		if err != nil {
			return nil, false, err
		}
		if prior != nil {
			return prior, true, nil
		}
	}
	out, err := s.createDirect(ctx, actorID, branchID, in)
	if err != nil && in.Doc.ClientRef != "" && dberr.IsDuplicate(err) {
		if prior, perr := s.directByClientRef(ctx, branchID, in.Doc.ClientRef); perr == nil && prior != nil {
			return prior, true, nil
		}
	}
	return out, false, err
}

// directByClientRef returns the direct note created under a client reference,
// nil when none; a reference that names an order note is a conflict.
func (s *Service) directByClientRef(ctx context.Context, branchID int64, ref string) (*contracts.DeliveryNote, error) {
	id, err := s.repo.NoteIDByClientRef(ctx, branchID, ref)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if id == 0 {
		return nil, nil
	}
	n, err := s.GetByID(ctx, id)
	if err != nil || n == nil {
		return nil, err
	}
	if n.Origin != contracts.OriginDirect {
		return nil, apperror.Conflict("Referensi klien sudah dipakai untuk surat jalan lain")
	}
	return n, nil
}

func (s *Service) createDirect(ctx context.Context, actorID, branchID int64, in DirectInput) (*contracts.DeliveryNote, error) {
	var fields []apperror.FieldError
	fail := func(field, msg string) { fields = append(fields, apperror.FieldError{Field: field, Message: msg}) }
	if in.PartyID == 0 {
		fail("partyId", "pelanggan wajib dipilih (surat jalan langsung bukan untuk penjualan tunai tanpa pelanggan)")
	} else if p, err := s.parties.GetByID(ctx, in.PartyID); err != nil {
		return nil, apperror.Internal(err)
	} else if p == nil || p.Type != partycontracts.PartyCustomer || p.Status != "active" {
		fail("partyId", "customer tidak ditemukan")
	}
	deliveryDate := strings.TrimSpace(in.DeliveryDate)
	if deliveryDate != "" {
		if _, err := timeutil.ParseDateInput(deliveryDate); err != nil {
			fail("deliveryDate", "format tanggal harus YYYY-MM-DD")
		} else if timeutil.AfterTodayWIB(deliveryDate) {
			fail("deliveryDate", "tidak boleh di masa depan")
		}
	} else {
		deliveryDate = timeutil.TodayWIB()
	}
	if len(in.Lines) == 0 {
		fail("items", "wajib diisi")
	}
	if len(in.Doc.DriverName) > 100 {
		fail("driverName", "maksimal 100 karakter")
	}
	if len(in.Doc.VehiclePlate) > 20 {
		fail("vehiclePlate", "maksimal 20 karakter")
	}
	if len(in.Doc.WarehouseStaffName) > 100 {
		fail("warehouseStaffName", "maksimal 100 karakter")
	}
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	shipLabel, shipRecipient, shipPhone, shipAddr, err := s.resolveShipTo(ctx, in)
	if err != nil {
		return nil, err
	}
	items, names, err := s.directLines(ctx, branchID, in.Lines)
	if err != nil {
		return nil, err
	}
	number, err := s.branches.NextDocumentNumber(ctx, branchID, branchcontracts.DocDeliveryNote)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	id, err := s.repo.CreateNote(ctx, &contracts.DeliveryNote{
		Number: number, BranchID: branchID, SalesOrderID: 0,
		Origin: contracts.OriginDirect, PartyID: in.PartyID,
		ShipToLabel: shipLabel, ShipToRecipient: shipRecipient, ShipToPhone: shipPhone, ShipToAddress: shipAddr,
		DeliveryDate:       deliveryDate,
		DriverName:         strings.TrimSpace(in.Doc.DriverName),
		VehiclePlate:       strings.TrimSpace(in.Doc.VehiclePlate),
		WarehouseStaffName: strings.TrimSpace(in.Doc.WarehouseStaffName),
		DropLocationNote:   strings.TrimSpace(in.Doc.DropLocationNote),
		Notes:              in.Notes, Items: items, ClientRef: in.Doc.ClientRef,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "deliveries.create_direct", Entity: "delivery", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number,
		Metadata: map[string]any{"partyId": in.PartyID}})
	out, err := s.GetByID(ctx, id)
	if err != nil || out == nil {
		return out, err
	}
	out.Warnings = s.draftWarnings(ctx, branchID, nil, items, names)
	// The customer may already have shipments waiting for their order: say so,
	// so the admin bundles them into one order instead of forgetting them.
	if sum, err := s.repo.OpenDirectSummary(ctx, branchID, in.PartyID); err == nil && sum.Count > 0 {
		out.Warnings = append(out.Warnings, "Pelanggan ini masih punya "+strconv.FormatInt(sum.Count, 10)+
			" surat jalan langsung yang belum diorder (tertua "+sum.OldestDate+"); buat ordernya dari menu Order Jual.")
	}
	return out, nil
}

// resolveShipTo snapshots the ship-to of a direct note: a saved customer
// address wins, else the ad-hoc fields as typed (any detail needs the address
// text), else nothing.
func (s *Service) resolveShipTo(ctx context.Context, in DirectInput) (label, recipient, phone, address string, err error) {
	if in.ShipToAddressID != 0 {
		a, err := s.parties.GetAddress(ctx, in.PartyID, in.ShipToAddressID)
		if err != nil {
			return "", "", "", "", apperror.Internal(err)
		}
		if a == nil {
			return "", "", "", "", apperror.Validation("", []apperror.FieldError{{Field: "shipToAddressId", Message: "alamat kirim tidak ditemukan"}})
		}
		return a.Label, a.Recipient, a.Phone, a.Text, nil
	}
	label, recipient = strings.TrimSpace(in.ShipToLabel), strings.TrimSpace(in.ShipToRecipient)
	phone, address = strings.TrimSpace(in.ShipToPhone), strings.TrimSpace(in.ShipToAddress)
	if label == "" && recipient == "" && phone == "" && address == "" {
		return "", "", "", "", nil
	}
	var fields []apperror.FieldError
	fail := func(field, msg string) { fields = append(fields, apperror.FieldError{Field: field, Message: msg}) }
	if address == "" {
		fail("shipToAddress", "alamat kirim wajib diisi bila memakai alamat sekali pakai")
	}
	if len([]rune(address)) > maxShipToAddress {
		fail("shipToAddress", "maksimal 1000 karakter")
	}
	if len([]rune(label)) > maxShipToLabel {
		fail("shipToLabel", "maksimal 120 karakter")
	}
	if len([]rune(recipient)) > maxShipToRecipient {
		fail("shipToRecipient", "maksimal 150 karakter")
	}
	if len([]rune(phone)) > maxShipToPhone {
		fail("shipToPhone", "maksimal 30 karakter")
	}
	if len(fields) > 0 {
		return "", "", "", "", apperror.Validation("", fields)
	}
	return label, recipient, phone, address, nil
}

// directLines validates the lines of a direct note against the catalog (one
// batch read): active stock-tracked goods, a known unit, a positive quantity,
// a location or an allocation across the branch. It returns the stored items
// and the product names (for the draft warnings).
func (s *Service) directLines(ctx context.Context, branchID int64, lines []LineInput) ([]*contracts.Item, map[string]string, error) {
	productIDs := make([]int64, 0, len(lines))
	for _, l := range lines {
		productIDs = append(productIDs, l.ProductID)
	}
	loaded, err := s.products.GetByIDs(ctx, productIDs)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	items := make([]*contracts.Item, 0, len(lines))
	names := map[string]string{}
	seen := map[string]bool{}
	for i, line := range lines {
		tag := "baris " + strconv.Itoa(i+1) + ": "
		if line.Qty <= 0 {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "qty harus lebih dari 0"}})
		}
		p := loaded[line.ProductID]
		if p == nil || p.Status != "active" {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "produk tidak ditemukan"}})
		}
		if p.Type != productcontracts.TypeBarang || !p.Tracked {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items",
				Message: tag + "produk " + p.Name + " bukan barang berstok; jasa/non-stok ditambahkan di order, bukan di surat jalan langsung"}})
		}
		known := false
		for _, v := range p.Variants {
			if v.ID == line.VariantID {
				known = true
				break
			}
		}
		if !known {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "varian tidak dikenal"}})
		}
		uom := strings.TrimSpace(line.UOM)
		factor, _, ok := p.UOMFactor(uom)
		if !ok {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "satuan tidak dikenal"}})
		}
		key := lineKey(line.ProductID, line.VariantID) + "/" + uom + "/" + strconv.FormatInt(line.LocationID, 10)
		if seen[key] {
			return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: tag + "baris ganda (produk/varian/satuan/lokasi sama)"}})
		}
		seen[key] = true
		names[lineKey(line.ProductID, line.VariantID)] = p.Name
		qtyBase := line.Qty * factor
		if line.LocationID == 0 {
			sug, short, err := s.stock.Allocate(ctx, branchID, line.ProductID, line.VariantID, qtyBase)
			if err != nil {
				return nil, nil, err
			}
			if short > 1e-9 {
				return nil, nil, apperror.Conflict("Stok tidak mencukupi (" + tag + "kurang " + strconv.FormatFloat(short, 'f', -1, 64) + ")")
			}
			for _, a := range sug {
				items = append(items, &contracts.Item{
					ProductID: line.ProductID, VariantID: line.VariantID, LocationID: a.LocationID,
					UOM: uom, UOMFactor: factor, Qty: a.Qty / factor, QtyBase: a.Qty,
				})
			}
			continue
		}
		if err := s.stock.CheckLocation(ctx, branchID, line.LocationID); err != nil {
			return nil, nil, err
		}
		items = append(items, &contracts.Item{
			ProductID: line.ProductID, VariantID: line.VariantID, LocationID: line.LocationID,
			UOM: uom, UOMFactor: factor, Qty: line.Qty, QtyBase: qtyBase,
		})
	}
	return items, names, nil
}

// confirmDirect ships an unordered direct note: evidence, one idempotent
// stock posting, one CAS draft -> confirmed. Serialized per note; a confirmed
// note replays as success.
func (s *Service) confirmDirect(ctx context.Context, actorID, branchID, id int64, in ConfirmInput) (*contracts.DeliveryNote, error) {
	if in.Settlement != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "settlement", Message: "tidak berlaku untuk surat jalan langsung (belum ada order)"}})
	}
	err := doclock.Lock(noteLock(id), func() error {
		n, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		switch n.Status {
		case contracts.StatusConfirmed:
			return nil
		case contracts.StatusDraft:
		default:
			return apperror.Conflict("Hanya draft yang bisa dikonfirmasi")
		}
		if !isUnorderedDirect(n) {
			return apperror.Conflict("Status surat jalan berubah, muat ulang")
		}
		full, err := s.enrich(ctx, n)
		if err != nil {
			return err
		}
		if err := s.requireEvidence(ctx, nil, full, in); err != nil {
			return err
		}
		if err := s.postNote(ctx, branchID, actorID, full, nil); err != nil {
			return err
		}
		won, err := s.repo.Confirm(ctx, full.ID, &contracts.DeliveryNote{
			RecipientName:                   strings.TrimSpace(in.RecipientName),
			RecipientSignatureStatus:        strings.TrimSpace(in.RecipientSignatureStatus),
			RecipientSignatureMissingReason: strings.TrimSpace(in.RecipientSignatureMissingReason),
		}, actorID, s.confirmAudit(branchID, actorID, full))
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			cur, err := s.repo.GetNote(ctx, full.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if cur == nil || cur.Status != contracts.StatusConfirmed {
				return apperror.Conflict("Status surat jalan berubah, muat ulang")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// cancelDirect voids an unordered direct note, draft or confirmed: the stock
// it moved (if any) is reversed first (idempotent), then one CAS flips it with
// the reason and the audit row in the same transaction. A note already
// cancelled replays as success; a note that got its order meanwhile is refused
// (corrections then go through a sales return).
func (s *Service) cancelDirect(ctx context.Context, actorID, branchID, id int64, reason string) (*contracts.DeliveryNote, error) {
	reason, err := validator.Reason(reason, false)
	if err != nil {
		return nil, err
	}
	err = doclock.Lock(noteLock(id), func() error {
		cur, err := s.mustOwnBranch(ctx, id, branchID)
		if err != nil {
			return err
		}
		if cur.Status == contracts.StatusCancelled {
			return nil
		}
		if !isUnorderedDirect(cur) {
			return apperror.Conflict("Surat jalan sudah punya order; perbaiki lewat retur penjualan")
		}
		moved, err := s.stock.HasMovements(ctx, branchID, stockcontracts.RefDelivery, cur.ID)
		if err != nil {
			return err
		}
		if moved {
			if _, err := s.stock.Reverse(ctx, branchID, stockcontracts.RefDelivery, cur.ID, actorID); err != nil {
				return err
			}
		}
		won, err := s.repo.CancelDirect(ctx, cur.ID, reason, actorID, func(ctx context.Context, tx *infrastructure.Tx) error {
			note := cur.Number
			if reason != "" {
				note += ": " + reason
			}
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "deliveries.cancel", Entity: "delivery", EntityID: cur.ID, BranchID: branchID, ActorID: actorID, Note: note,
				Before: map[string]any{"status": cur.Status},
				After:  map[string]any{"status": contracts.StatusCancelled, "reason": reason, "origin": contracts.OriginDirect},
			})
		})
		if err != nil {
			return apperror.Internal(err)
		}
		if !won {
			latest, err := s.repo.GetNote(ctx, cur.ID)
			if err != nil {
				return apperror.Internal(err)
			}
			if latest == nil || latest.Status != contracts.StatusCancelled {
				return apperror.Conflict("Status surat jalan berubah, muat ulang")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// GetByIDs implements the contract: headers + lines in two grouped reads,
// customer names of direct notes in a third.
func (s *Service) GetByIDs(ctx context.Context, ids []int64) (map[int64]*contracts.DeliveryNote, error) {
	notes, err := s.repo.NotesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(notes) == 0 {
		return notes, nil
	}
	noteIDs := make([]int64, 0, len(notes))
	for id := range notes {
		noteIDs = append(noteIDs, id)
	}
	lines, err := s.repo.ItemsByDeliveries(ctx, noteIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	list := make([]*contracts.DeliveryNote, 0, len(notes))
	for id, n := range notes {
		n.Items = lines[id]
		if n.Items == nil {
			n.Items = []*contracts.Item{}
		}
		list = append(list, n)
	}
	if err := s.decorateParties(ctx, list); err != nil {
		return nil, err
	}
	return notes, nil
}

// ListOpenDirect implements the contract (see there).
func (s *Service) ListOpenDirect(ctx context.Context, branchID, partyID int64, limit int) ([]*contracts.DeliveryNote, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	notes, err := s.repo.ListOpenDirect(ctx, branchID, partyID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if err := s.decorateParties(ctx, notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// OpenDirectSummary implements the contract (see there).
func (s *Service) OpenDirectSummary(ctx context.Context, branchID, partyID int64) (contracts.OpenDirectSummary, error) {
	sum, err := s.repo.OpenDirectSummary(ctx, branchID, partyID)
	if err != nil {
		return contracts.OpenDirectSummary{}, apperror.Internal(err)
	}
	return sum, nil
}

// decorateParties names the customers of the direct notes in one grouped read
// (order notes keep their order's customer). Best-effort: unknown ids stay
// unnamed.
func (s *Service) decorateParties(ctx context.Context, notes []*contracts.DeliveryNote) error {
	var ids []int64
	seen := map[int64]bool{}
	for _, n := range notes {
		if n.Origin == contracts.OriginDirect && n.PartyID != 0 && !seen[n.PartyID] {
			seen[n.PartyID] = true
			ids = append(ids, n.PartyID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := s.parties.NamesByIDs(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, n := range notes {
		if n.Origin == contracts.OriginDirect {
			n.PartyName = names[n.PartyID]
		}
	}
	return nil
}

// LinkDirect implements the contract (see there). Locks: the order first,
// then every note in ascending id — the same order every direct-note write
// takes, so a confirm or cancel racing a link cannot deadlock it.
func (s *Service) LinkDirect(ctx context.Context, actorID, branchID, soID int64, noteIDs []int64) error {
	ids := make([]int64, 0, len(noteIDs))
	seen := map[int64]bool{}
	for _, id := range noteIDs {
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "deliveryIds", Message: "wajib diisi"}})
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	keys := make([]string, 0, len(ids)+1)
	keys = append(keys, "so:"+strconv.FormatInt(soID, 10))
	for _, id := range ids {
		keys = append(keys, noteLock(id))
	}
	return lockAll(keys, func() error { return s.linkDirectLocked(ctx, actorID, branchID, soID, ids) })
}

// lockAll nests doclock.Lock over keys in the given order.
func lockAll(keys []string, fn func() error) error {
	if len(keys) == 0 {
		return fn()
	}
	return doclock.Lock(keys[0], func() error { return lockAll(keys[1:], fn) })
}

func (s *Service) linkDirectLocked(ctx context.Context, actorID, branchID, soID int64, ids []int64) error {
	so, err := s.sales.GetByID(ctx, soID)
	if err != nil {
		return apperror.Internal(err)
	}
	if so == nil || so.BranchID != branchID {
		return apperror.NotFound("Sales order")
	}
	if so.Origin != salescontracts.OriginDelivery {
		return apperror.Conflict("Order " + so.Number + " bukan order dari surat jalan")
	}
	if so.Status != salescontracts.StatusConfirmed {
		return apperror.Conflict("Order " + so.Number + " tidak lagi terkonfirmasi")
	}
	notes, err := s.repo.NotesByIDs(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	lines, err := s.repo.ItemsByDeliveries(ctx, ids)
	if err != nil {
		return apperror.Internal(err)
	}
	shipped := map[string]float64{}
	var toLink []int64
	var numbers []string
	for _, id := range ids {
		n := notes[id]
		if n == nil || n.BranchID != branchID {
			return apperror.NotFound("Surat jalan")
		}
		switch {
		case n.Origin != contracts.OriginDirect || n.DocumentKind != contracts.DocumentKindOrder:
			return apperror.Conflict("Surat jalan " + n.Number + " bukan surat jalan langsung")
		case n.Status != contracts.StatusConfirmed:
			return apperror.Conflict("Surat jalan " + n.Number + " belum dikonfirmasi (atau sudah dibatalkan)")
		case n.PartyID != so.PartyID:
			return apperror.Conflict("Surat jalan " + n.Number + " milik pelanggan lain")
		case n.SalesOrderID == soID:
			// Replay of a half-finished link: already ours, nothing to flip.
		case n.SalesOrderID != 0:
			return apperror.Conflict("Surat jalan " + n.Number + " sudah diorder (order lain)")
		default:
			toLink = append(toLink, id)
		}
		numbers = append(numbers, n.Number)
		for _, it := range lines[id] {
			shipped[lineKey(it.ProductID, it.VariantID)] += it.QtyBase
		}
	}
	// The order must bill exactly what these notes shipped: every stock line
	// of the order is covered in full, and nothing shipped is left unbilled.
	ordered := map[string]float64{}
	for _, l := range so.Items {
		if l.StockTracked {
			ordered[lineKey(l.ProductID, l.VariantID)] += l.QtyBase
		}
	}
	for key, q := range ordered {
		if d := shipped[key] - q; d > 1e-9 || d < -1e-9 {
			return apperror.Conflict("Jumlah barang di order tidak sama dengan surat jalannya; buat ulang order dari surat jalan")
		}
	}
	for key := range shipped {
		if _, ok := ordered[key]; !ok {
			return apperror.Conflict("Surat jalan memuat barang yang tidak ada di order; buat ulang order dari surat jalan")
		}
	}
	if len(toLink) == 0 {
		return nil
	}
	lost, err := s.repo.LinkDirect(ctx, soID, toLink, actorID, func(ctx context.Context, tx *infrastructure.Tx) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "deliveries.link_order", Entity: "sales_order", EntityID: soID, BranchID: branchID, ActorID: actorID,
			Note:     so.Number + " ← " + strings.Join(numbers, ", "),
			Metadata: map[string]any{"deliveryIds": toLink, "orderId": soID},
		})
	})
	if err != nil {
		return apperror.Internal(err)
	}
	if lost != 0 {
		cur, err := s.repo.GetNote(ctx, lost)
		if err != nil {
			return apperror.Internal(err)
		}
		if cur != nil && cur.SalesOrderID == soID {
			return nil // a twin of this very link won the race
		}
		number := strconv.FormatInt(lost, 10)
		if cur != nil {
			number = cur.Number
		}
		return apperror.Conflict("Surat jalan " + number + " berubah (sudah diorder atau dibatalkan); muat ulang")
	}
	return nil
}
