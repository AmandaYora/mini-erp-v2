package integration

import (
	"testing"

	partyapp "mini-erp/internal/modules/party/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// TestSalesOrderAdHocShipTo (SL-01): a one-off address typed on the order is
// snapshotted with address id 0 (customer or walk-in), survives a header-light
// edit, is validated, and a saved address id still wins over typed text.
func TestSalesOrderAdHocShipTo(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	line := []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}}

	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		ShipToLabel: "Proyek", ShipToRecipient: "Pak Joko", ShipToPhone: "0812345678",
		ShipToAddress: "Jl. Sementara 5, Blok C", Items: line,
	})
	if err != nil {
		t.Fatalf("create with ad-hoc ship-to: %v", err)
	}
	if so.ShipToAddressID != 0 || so.ShipToLabel != "Proyek" || so.ShipToRecipient != "Pak Joko" ||
		so.ShipToPhone != "0812345678" || so.ShipToAddress != "Jl. Sementara 5, Blok C" {
		t.Fatalf("ad-hoc snapshot wrong: %+v", so)
	}

	// Walk-in (no party) may carry an ad-hoc address as well.
	walk, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: 0, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		ShipToRecipient: "Tamu", ShipToAddress: "Jl. Lewat 1", Items: line,
	})
	if err != nil {
		t.Fatalf("walk-in ad-hoc: %v", err)
	}
	if walk.ShipToAddress != "Jl. Lewat 1" || walk.ShipToRecipient != "Tamu" {
		t.Fatalf("walk-in snapshot wrong: %+v", walk)
	}

	// A detail without the address text is refused.
	if _, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		ShipToRecipient: "Tanpa alamat", Items: line,
	}); err == nil {
		t.Fatal("expected refusal: recipient without address text")
	}

	// Update (draft) can change the typed address; clearing it removes the ship-to.
	up, err := m.sales.Service().UpdateOrder(ctx(), 0, fx.branchID, so.ID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		ShipToAddress: "Jl. Baru 9", Items: line,
	})
	if err != nil {
		t.Fatalf("update ad-hoc: %v", err)
	}
	if up.ShipToAddress != "Jl. Baru 9" || up.ShipToRecipient != "" || up.ShipToLabel != "" {
		t.Fatalf("update did not replace snapshot: %+v", up)
	}
	up, err = m.sales.Service().UpdateOrder(ctx(), 0, fx.branchID, so.ID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", Items: line,
	})
	if err != nil {
		t.Fatalf("update clear: %v", err)
	}
	if up.ShipToAddress != "" || up.ShipToAddressID != 0 {
		t.Fatalf("ship-to not cleared: %+v", up)
	}
}

// TestPOSOrderCarriesNotesAndShipTo (SL-07): a POS-channel order keeps the
// transaction note and the opt-in saved delivery address, snapshotted.
func TestPOSOrderCarriesNotesAndShipTo(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	addr, err := m.party.Service().CreateAddress(ctx(), 0, fx.customerID, "customer", partyapp.AddressInput{
		Label: "Rumah", Recipient: "Siti", Phone: "081234567890", Text: "Jl. Kenanga 3", IsPrimary: true,
	})
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "pos", PaymentTerms: "cod", TaxType: "none",
		Notes: "antar sore", ShipToAddressID: addr.ID, ClientRef: "pos-cart-1",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("create POS order: %v", err)
	}
	if so.Notes != "antar sore" || so.ShipToAddressID != addr.ID || so.ShipToAddress != "Jl. Kenanga 3" || so.ShipToRecipient != "Siti" {
		t.Fatalf("POS order = notes %q ship-to %d/%q/%q", so.Notes, so.ShipToAddressID, so.ShipToAddress, so.ShipToRecipient)
	}
}
