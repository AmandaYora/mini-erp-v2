// Package integrity carries the read-only consistency findings that modules
// report to the operator console (no auto-healing: the operator decides).
package integrity

// Issue is one document left in an in-between state by a failed multi-step flow.
type Issue struct {
	Kind    string `json:"kind"`
	DocType string `json:"docType"`
	DocID   int64  `json:"docId"`
	Number  string `json:"number"`
	Detail  string `json:"detail"`
}
