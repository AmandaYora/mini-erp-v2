package contracts

// StockReportRow is one tracked product's total position in a branch.
type StockReportRow struct {
	ProductID   int64   `json:"productId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	OnHand      float64 `json:"onHand"`
	Reserved    float64 `json:"reserved"`
	Available   float64 `json:"available"`
	MinStock    float64 `json:"minStock"`
	Critical    bool    `json:"critical"`
}

// StockReport is one page of the stock report.
type StockReport struct {
	Rows  []StockReportRow `json:"rows"`
	Total int64            `json:"total"`
}
