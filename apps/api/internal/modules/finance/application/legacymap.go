package application

// LegacySystemKeyToMappingKey maps the legacy chart's `system_key` values to
// this module's mapping keys (F2b-8). The data migration uses it so custom
// legacy accounts keep their meaning even when an accountant changed their
// codes: the key, not the code, identifies what an account is for.
var LegacySystemKeyToMappingKey = map[string]string{
	"cash_cash":                  "cash",
	"cash_bank":                  "bank",
	"accounts_receivable":        "receivable",
	"inventory":                  "inventory",
	"input_tax":                  "ppn_in",
	"accounts_payable":           "payable",
	"output_tax":                 "ppn_out",
	"customer_advance":           "customer_advance",
	"supplier_advance":           "supplier_advance",
	"opening_equity":             "opening_equity",
	"sales_revenue":              "revenue",
	"cogs":                       "cogs",
	"stock_difference":           "adjustment",
	"rounding_difference":        "rounding_difference",
	"purchase_return_difference": "purchase_return_difference",
}
