package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/shared/apperror"
)

// Scope bounds what one assistant run may read (AS-01).
//
// The accessible set comes from the caller's identity, never from the message:
//   - Restricted=false: system callers and whitelisted WhatsApp numbers. The
//     authorization list is company-wide (a number is not linked to a user or a
//     branch), so every branch is reachable — the pre-AS-01 behaviour.
//   - Restricted=true: a signed-in operator (console/simulate). Only the
//     branches in Allowed (the user's own branch access) may be named, picked
//     or aggregated; "all branches" means all of THEIRS.
type Scope struct {
	// BranchID is an explicit pick (console branch picker); 0 = none, resolve
	// from the text or fall back to every accessible active branch.
	BranchID   int64
	Restricted bool
	Allowed    []int64
}

func (s Scope) allows(id int64) bool {
	if !s.Restricted {
		return true
	}
	for _, a := range s.Allowed {
		if a == id {
			return true
		}
	}
	return false
}

// Outcome is one engine run: the reply plus the branch it was narrowed to
// (BranchID 0 / empty name = several branches).
type Outcome struct {
	Intent     string
	Answer     string
	Calls      []ToolCall
	BranchID   int64
	BranchName string
}

// Run resolves the branch scope, then answers. A tool failure returns the
// outcome (with the recorded calls) and the error.
func (e *Engine) Run(ctx context.Context, sc Scope, text string) (*Outcome, error) {
	d := Detect(text)
	all, err := e.branches.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := &Outcome{Intent: d.Intent}

	var working []*branchcontracts.Branch
	narrowed := false
	if sc.BranchID != 0 {
		b := branchByID(all, sc.BranchID)
		if b == nil {
			return nil, apperror.NotFound("Cabang")
		}
		if !sc.allows(b.ID) {
			return nil, apperror.Forbidden()
		}
		working, narrowed = []*branchcontracts.Branch{b}, true
	} else {
		var accessible []*branchcontracts.Branch
		for _, b := range all {
			if b.Status == "active" && sc.allows(b.ID) {
				accessible = append(accessible, b)
			}
		}
		working = accessible
		if d.Intent != IntentHelp {
			probe := stripNonBranchTokens(text)
			named := matchBranches(probe, accessible)
			switch {
			case len(named) > 1:
				out.Answer = clarifyBranch(named)
				return out, nil
			case len(named) == 1:
				working, narrowed = named, true
			case sc.Restricted:
				// A branch the caller cannot access must not silently turn
				// into "your other branches" (the answer would be about
				// something they did not ask for), nor leak its figures.
				var hidden []*branchcontracts.Branch
				for _, b := range all {
					if b.Status == "active" && !sc.allows(b.ID) {
						hidden = append(hidden, b)
					}
				}
				if h := matchBranches(probe, hidden); len(h) > 0 {
					out.Answer = fmt.Sprintf("Kamu tidak punya akses ke cabang %s.", h[0].Name)
					return out, nil
				}
			}
			if len(working) == 0 {
				out.Answer = "Kamu belum punya akses ke cabang aktif mana pun."
				return out, nil
			}
		}
	}

	rec := newRecorder()
	intent, answer, err := e.dispatch(ctx, d, working, text, rec)
	out.Intent, out.Answer, out.Calls = intent, answer, rec.calls
	if narrowed && len(working) == 1 {
		out.BranchID, out.BranchName = working[0].ID, working[0].Name
		if intent != IntentHelp && err == nil {
			out.Answer = "Cabang: " + working[0].Name + "\n" + answer
		}
	}
	return out, err
}

// CheckScope rejects an explicit branch that does not exist (404) or that the
// caller may not use (403) before a run is recorded: the console picker only
// lists real, accessible branches, so anything else is a tampered request.
func (e *Engine) CheckScope(ctx context.Context, sc Scope) error {
	if sc.BranchID == 0 {
		return nil
	}
	all, err := e.branches.ListAll(ctx)
	if err != nil {
		return err
	}
	b := branchByID(all, sc.BranchID)
	if b == nil {
		return apperror.NotFound("Cabang")
	}
	if !sc.allows(b.ID) {
		return apperror.Forbidden()
	}
	return nil
}

func branchByID(all []*branchcontracts.Branch, id int64) *branchcontracts.Branch {
	for _, b := range all {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// genericBranchWords are dropped from the front of a branch name to get its
// distinctive part ("Cabang Bandung" is also "Bandung").
var genericBranchWords = map[string]bool{"cabang": true, "toko": true, "outlet": true, "gudang": true}

// words lowers text into letter/digit tokens ("Bandung, hari-ini" ->
// "bandung hari ini") so matching is whole-word, never a substring of a word.
func words(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// stripNonBranchTokens removes fragments that look like branch names but are
// not: an order number carries the branch CODE ("ORD-UTM/PJ/…") and a quoted
// fragment is a product name. Neither may pick a branch.
func stripNonBranchTokens(text string) string {
	if num := findOrderNumber(text); num != "" {
		text = strings.ReplaceAll(strings.ToUpper(text), num, " ")
	}
	if q := quoted(text); q != "" {
		text = strings.Replace(text, q, " ", 1)
	}
	return text
}

// branchCandidates are the phrases that identify a branch in a message: name,
// code, city, and the name without a generic lead word. Single characters are
// too noisy to match.
func branchCandidates(b *branchcontracts.Branch) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		w := words(raw)
		if len(w) < 2 || seen[w] {
			return
		}
		seen[w] = true
		out = append(out, w)
	}
	add(b.Name)
	add(b.Code)
	add(b.City)
	if toks := strings.Fields(words(b.Name)); len(toks) > 1 && genericBranchWords[toks[0]] {
		add(strings.Join(toks[1:], " "))
	}
	return out
}

// matchBranches returns the distinct branches the text names. When one branch's
// matched phrase contains another's ("Jakarta Selatan" vs "Jakarta"), the more
// specific branch wins; remaining matches are ambiguous.
func matchBranches(text string, branches []*branchcontracts.Branch) []*branchcontracts.Branch {
	padded := " " + words(text) + " "
	type hit struct {
		b    *branchcontracts.Branch
		best string
	}
	var hits []hit
	for _, b := range branches {
		best := ""
		for _, c := range branchCandidates(b) {
			if strings.Contains(padded, " "+c+" ") && len(c) > len(best) {
				best = c
			}
		}
		if best != "" {
			hits = append(hits, hit{b, best})
		}
	}
	var out []*branchcontracts.Branch
	for i, h := range hits {
		dominated := false
		for j, o := range hits {
			if i != j && len(o.best) > len(h.best) && strings.Contains(" "+o.best+" ", " "+h.best+" ") {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, h.b)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func clarifyBranch(named []*branchcontracts.Branch) string {
	var b strings.Builder
	b.WriteString("Cabang mana yang kamu maksud? Ada beberapa yang cocok:\n")
	for _, br := range named {
		label := br.Name
		if br.City != "" {
			label += " (" + br.City + ")"
		}
		fmt.Fprintf(&b, "- %s\n", label)
	}
	b.WriteString("Ulangi pertanyaannya dengan menyebut salah satu nama atau kode cabang.")
	return b.String()
}
