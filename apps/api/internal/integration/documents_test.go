package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	partyapp "mini-erp/internal/modules/party/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// TestSalesOrderShipToSnapshot locks the C2 contract: the address freezes
// onto the order at write time and never follows later master edits.
func TestSalesOrderShipToSnapshot(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	addr, err := m.party.Service().CreateAddress(ctx(), 0, fx.customerID, "customer", partyapp.AddressInput{
		Label: "Gudang", Recipient: "Budi", Phone: "081234567890", Text: "Jl. Mawar 1", IsPrimary: true,
	})
	if err != nil {
		t.Fatalf("create address: %v", err)
	}
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "none", ShipToAddressID: addr.ID,
		TaxInvoiceNumber: "010.000-26.00000001",
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if so.ShipToAddressID != addr.ID || so.ShipToLabel != "Gudang" ||
		so.ShipToRecipient != "Budi" || so.ShipToPhone != "081234567890" ||
		so.ShipToAddress != "Jl. Mawar 1" {
		t.Fatalf("snapshot not frozen: %+v", so)
	}
	if so.TaxInvoiceNumber != "010.000-26.00000001" {
		t.Fatalf("tax invoice not stored: %+v", so)
	}

	// Master edit must not move the snapshot.
	if _, err := m.party.Service().UpdateAddress(ctx(), 0, fx.customerID, "customer", addr.ID, partyapp.AddressInput{
		Label: "Kantor", Recipient: "Ani", Phone: "082234567890", Text: "Jl. Melati 9", IsPrimary: true,
	}); err != nil {
		t.Fatalf("update address: %v", err)
	}
	again, err := m.sales.Service().GetByID(ctx(), so.ID)
	if err != nil {
		t.Fatalf("get SO: %v", err)
	}
	if again.ShipToLabel != "Gudang" || again.ShipToRecipient != "Budi" ||
		again.ShipToAddress != "Jl. Mawar 1" {
		t.Fatalf("snapshot moved with master: %+v", again)
	}

	// Unknown address id is rejected, never snapshotted as zero-row.
	_, err = m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "none", ShipToAddressID: 99999,
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1,
		}},
	})
	if err == nil {
		t.Fatal("expected rejection for unknown address, got nil")
	}
}

// TestDeliveryConfirmRecipientRules locks the C4 receipt validation:
// missing signature without reason is rejected; bare confirms still pass.
func TestDeliveryConfirmRecipientRules(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	// One SO with room for four 1-unit test deliveries.
	so, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
		TaxType: "none",
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 10,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	mkNote := func() int64 {
		t.Helper()
		dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "",
			deliveryapp.DocInput{},
			[]deliveryapp.LineInput{{
				ProductID: fx.productID, VariantID: fx.variantID,
				LocationID: fx.locationID, UOM: "pcs", Qty: 1,
			}})
		if err != nil {
			t.Fatalf("create delivery: %v", err)
		}
		return dn.ID
	}

	id := mkNote()
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, id, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("bare confirm: %v", err)
	}
	id = mkNote()
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, id, deliveryapp.ConfirmInput{
		RecipientName: "Ani", RecipientSignatureStatus: "bogus",
	}); err == nil {
		t.Fatal("expected rejection for bogus status, got nil")
	}
	id = mkNote()
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, id, deliveryapp.ConfirmInput{
		RecipientSignatureStatus: "missing",
	}); err == nil {
		t.Fatal("expected rejection for missing reason, got nil")
	}
	id = mkNote()
	done, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, id, deliveryapp.ConfirmInput{
		RecipientName: "Ani", RecipientSignatureStatus: "missing",
		RecipientSignatureMissingReason: "toko tutup",
	})
	if err != nil {
		t.Fatalf("missing-with-reason confirm: %v", err)
	}
	if done.RecipientName != "Ani" || done.RecipientSignatureMissingReason != "toko tutup" {
		t.Fatalf("receipt not persisted: %+v", done)
	}
}
