package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Party types in the single parties table.
const (
	PartyCustomer = "customer"
	PartySupplier = "supplier"
)

// Party is a customer or supplier.
type Party struct {
	ID           int64
	Code         string
	Type         string // customer | supplier
	Name         string
	Phone        string
	Email        string
	Address      string
	Notes        string
	MemberTypeID *int64
	MemberCode   string
	Status       string // active | archived
}

// Address is one ship-to address of a customer.
type Address struct {
	ID        int64
	PartyID   int64
	Label     string
	Recipient string
	Phone     string
	Text      string
	IsPrimary bool
	SortOrder int
	Status    string // active | archived
}

// MemberType groups customers with a special price rule (one rule per type —
// columns, not a rules table; DB_SCHEMA §6).
type MemberType struct {
	ID             int64
	Code           string
	Name           string
	Description    string
	PriceBasis     string // selling_price | min_selling_price | purchase_price
	Direction      string // minus | plus
	AdjustmentType string // percent | nominal
	Adjustment     float64
	RoundingMode   string // none | round_100 | round_500 | round_1000 | floor | ceil
	RoundingStep   float64
	Status         string // active | archived
}

// QuoteLine is one priced line.
type QuoteLine struct {
	ProductID      int64 `json:"productId"`
	StandardPrice  int64 `json:"standardPrice"`
	MemberPrice    int64 `json:"memberPrice"`
	Applied        bool  `json:"applied"`
	MemberInactive bool  `json:"memberInactive"`
}

// QuoteResult is the pricing answer for a cart.
type QuoteResult struct {
	MemberType     string      `json:"memberType"`
	MemberInactive bool        `json:"memberInactive"`
	Lines          []QuoteLine `json:"lines"`
}

// PartyClient is the public surface of the party module.
type PartyClient interface {
	// GetByID resolves a party with member code, or nil when missing.
	GetByID(ctx context.Context, id int64) (*Party, error)
	// GetAddress resolves one customer address scoped to its party
	// (nil when missing, foreign, or archived) for order snapshots.
	GetAddress(ctx context.Context, partyID, addressID int64) (*Address, error)
	// NamesByIDs resolves party names for many ids in one grouped read
	// (A3). Unknown ids are absent from the map — best-effort, never error.
	NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

// PricingClient quotes member prices (consumed by sales/POS).
type PricingClient interface {
	// Quote prices lines for a customer (0 = walk-in, standard prices).
	Quote(ctx context.Context, customerID int64, productIDs []int64) (*QuoteResult, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "customers.view", Name: "Customer: Lihat"},
		{Code: "customers.create", Name: "Customer: Tambah"},
		{Code: "customers.update", Name: "Customer: Ubah"},
		{Code: "customers.archive", Name: "Customer: Arsipkan"},
		{Code: "suppliers.view", Name: "Supplier: Lihat"},
		{Code: "suppliers.create", Name: "Supplier: Tambah"},
		{Code: "suppliers.update", Name: "Supplier: Ubah"},
		{Code: "suppliers.archive", Name: "Supplier: Arsipkan"},
		{Code: "member_types.manage", Name: "Tipe Member: Kelola"},
		{Code: "pricing.quote", Name: "Harga: Quote Member"},
	}
}
