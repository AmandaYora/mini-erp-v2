package contracts

import (
	"context"
	"time"

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
	// ArchivedAt is when the party was archived (nil while active/restored).
	ArchivedAt *time.Time
	// Attributes is the free-form "Informasi Tambahan" key/value block
	// (legacy metadata_json). Never nil on read.
	Attributes map[string]string
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
	RoundingMode   string // none | nearest | up | down (step = kelipatan)
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
	// Error is set when THIS line could not be priced (unknown/archived
	// product, empty basis price, invalid unit conversion, negative result).
	// One bad line never fails the whole quote; the line then carries the
	// standard price and Applied=false.
	Error string `json:"error,omitempty"`
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
	// LabelsByIDs resolves code+name for many parties in one grouped read,
	// for readers that show both (receivables/payables list a party by code
	// and name). Unknown ids are absent — best-effort, never error.
	LabelsByIDs(ctx context.Context, ids []int64) (map[int64]Label, error)
	// SearchIDs resolves a free-text term (code, name, phone, email, address)
	// to party ids so another module can filter its own documents by party
	// without a join. partyType "" = customers and suppliers. The term must
	// have at least 2 characters (shorter returns none); the result is capped
	// at limit and never exceeds MaxSearchIDs (decision A-15: a broader term
	// is truncated, the caller narrows the term).
	SearchIDs(ctx context.Context, partyType, term string, limit int) ([]int64, error)
}

// MaxSearchIDs caps PartyClient.SearchIDs.
const MaxSearchIDs = 1000

// MinSearchChars is the shortest term SearchIDs accepts.
const MinSearchChars = 2

// Label is a party's identity for report decoration: code and name, without
// the full party row behind them.
type Label struct {
	Code string
	Name string
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
		{Code: "member_types.view", Name: "Tipe Member: Lihat"},
		{Code: "member_types.manage", Name: "Tipe Member: Kelola"},
		{Code: "pricing.quote", Name: "Harga: Quote Member"},
	}
}
