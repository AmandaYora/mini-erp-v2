// Package wiring holds the tiny technical helper behind Modules.Verify: it
// reports late-bound dependencies that the composition root forgot to set, so
// the mistake fails at startup (and in the wiring test) instead of at the
// first request that happens to need them.
package wiring

import (
	"fmt"
	"strings"
)

// Dep names one late-bound dependency and whether it has been set.
type Dep struct {
	Name string
	Set  bool
}

// Check returns nil when every dependency is set, else an error naming the
// module and each missing dependency.
func Check(module string, deps ...Dep) error {
	var missing []string
	for _, d := range deps {
		if !d.Set {
			missing = append(missing, d.Name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s: late-bound dependencies not wired: %s", module, strings.Join(missing, ", "))
}
