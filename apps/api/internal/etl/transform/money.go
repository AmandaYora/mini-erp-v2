// Package transform turns extracted legacy rows into revamp rows. It is pure:
// no database access, only the raw tables in and canonical rows out, so the
// same input always yields byte-identical output (the determinism test).
package transform

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// RoundHalfAway converts an exact decimal to integer rupiah, rounding half
// away from zero (the same rule as math.Round in shared/money). It is the one
// conversion path for money (design F5-4); there is no float anywhere.
func RoundHalfAway(r *big.Rat) int64 {
	if r.Sign() == 0 {
		return 0
	}
	neg := r.Sign() < 0
	num := new(big.Int).Abs(r.Num())
	den := r.Denom()
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	n := q.Int64()
	if neg {
		return -n
	}
	return n
}

// Rounder rounds money and writes every changed value to the adjustment log.
type Rounder struct{ Log *report.Log }

// Round converts v to rupiah; a non-zero fractional part is logged (D-ETL-8).
func (m *Rounder) Round(entity string, id int64, field string, v *big.Rat) int64 {
	n := RoundHalfAway(v)
	if v.Cmp(new(big.Rat).SetInt64(n)) != 0 && m.Log != nil {
		m.Log.Adjust(entity, id, field, v.FloatString(4), fmt.Sprint(n), "round_half_away")
	}
	return n
}

// R is shorthand for a column of a row.
func (m *Rounder) R(entity string, id int64, field string, r row.Row, col string) int64 {
	return m.Round(entity, id, field, r.Rat(col))
}

// SplitByQty divides total rupiah over parts in proportion to their quantity
// (largest remainder: Σ result == total exactly, ties go to the earlier part).
// A zero Σ qty yields all zeros except total on the first part.
func SplitByQty(total int64, qtys []*big.Rat) []int64 {
	out := make([]int64, len(qtys))
	if len(qtys) == 0 {
		return out
	}
	sum := new(big.Rat)
	for _, q := range qtys {
		sum.Add(sum, q)
	}
	if sum.Sign() == 0 {
		out[0] = total
		return out
	}
	neg := total < 0
	abs := total
	if neg {
		abs = -abs
	}
	type frac struct {
		i   int
		rem *big.Rat
	}
	fr := make([]frac, len(qtys))
	left := abs
	for i, q := range qtys {
		share := new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).SetInt64(abs), q), sum)
		fl := new(big.Int).Quo(share.Num(), share.Denom()) // shares are ≥ 0: Quo floors
		out[i] = fl.Int64()
		left -= out[i]
		fr[i] = frac{i, new(big.Rat).Sub(share, new(big.Rat).SetInt(fl))}
	}
	sort.SliceStable(fr, func(a, b int) bool { return fr[a].rem.Cmp(fr[b].rem) > 0 })
	for k := 0; left > 0; k = (k + 1) % len(fr) {
		out[fr[k].i]++
		left--
	}
	if neg {
		for i := range out {
			out[i] = -out[i]
		}
	}
	return out
}

// Line is one journal line in rupiah.
type Line struct{ Debit, Credit int64 }

// ErrBalanceTooLarge means the imbalance is data corruption, not rounding.
var ErrBalanceTooLarge = errors.New("selisih jurnal melebihi batas pembulatan")

// BalanceEntry forces Σdebit == Σcredit after rounding (design K4). The whole
// difference goes onto ONE line: the largest on the side that is short. It
// returns the index changed and the delta (signed: +delta added to that line).
// A difference above maxDelta is refused: that is a broken journal.
func BalanceEntry(lines []Line, maxDelta int64) (idx int, delta int64, side string, err error) {
	var d, c int64
	for _, l := range lines {
		d += l.Debit
		c += l.Credit
	}
	if d == c {
		return -1, 0, "", nil
	}
	diff := d - c // >0: credits are short
	abs := diff
	if abs < 0 {
		abs = -abs
	}
	if abs > maxDelta {
		return -1, 0, "", fmt.Errorf("%w: |%d| > %d", ErrBalanceTooLarge, abs, maxDelta)
	}
	best, bestVal := -1, int64(-1)
	for i, l := range lines {
		v := l.Credit
		if diff < 0 {
			v = l.Debit
		}
		if v > bestVal {
			best, bestVal = i, v
		}
	}
	if best < 0 || bestVal <= 0 {
		return -1, 0, "", errors.New("tidak ada baris di sisi yang kurang untuk menyeimbangkan")
	}
	if diff > 0 {
		lines[best].Credit += diff
		return best, diff, "credit", nil
	}
	lines[best].Debit += -diff
	return best, -diff, "debit", nil
}
