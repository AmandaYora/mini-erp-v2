package listquery

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"mini-erp/internal/shared/apperror"
)

func TestEmptyWhere(t *testing.T) {
	sql, args := New().SQL()
	if sql != "1=1" || len(args) != 0 {
		t.Fatalf("got %q %v", sql, args)
	}
}

func TestEqSkipsZero(t *testing.T) {
	w := New().Eq("branch_id", int64(3)).Eq("channel", "").Eq("party_id", int64(0)).Eq("status", "x")
	sql, args := w.SQL()
	if sql != "branch_id = ? AND status = ?" || !reflect.DeepEqual(args, []any{int64(3), "x"}) {
		t.Fatalf("got %q %v", sql, args)
	}
}

func TestPlaceholderOrderAndLike(t *testing.T) {
	w := New().Eq("branch_id", int64(1)).
		LikeOrIn("budi 50%", []string{"number"}, "party_id", []int64{7, 8}).
		StatusCSV("status", "confirmed, completed,,confirmed")
	sql, args := w.SQL()
	want := "branch_id = ? AND (((number LIKE ?) AND (number LIKE ?)) OR party_id IN (?,?)) AND status IN (?,?)"
	if sql != want {
		t.Fatalf("sql\n got %q\nwant %q", sql, want)
	}
	wantArgs := []any{int64(1), "%budi%", `%50\%%`, int64(7), int64(8), "confirmed", "completed"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args %v want %v", args, wantArgs)
	}
	if strings.Count(sql, "?") != len(args) {
		t.Fatal("placeholder/arg mismatch")
	}
}

func TestLikeEscapes(t *testing.T) {
	_, args := New().Like("a_b%c\\d", "name").SQL()
	if args[0] != `%a\_b\%c\\d%` {
		t.Fatalf("got %v", args[0])
	}
}

func TestLikeOrInEmptyIDsNoEmptyIn(t *testing.T) {
	sql, _ := New().LikeOrIn("x", []string{"number"}, "party_id", nil).SQL()
	if strings.Contains(sql, "IN") {
		t.Fatalf("empty ids must not emit IN: %q", sql)
	}
}

func TestInCapAndEmpty(t *testing.T) {
	if sql, _ := New().In("id").SQL(); sql != "1=1" {
		t.Fatalf("empty In must add nothing: %q", sql)
	}
	ids := make([]int64, 1500)
	_, args := New().InInt64("id", ids).SQL()
	if len(args) != MaxIn {
		t.Fatalf("In must truncate to %d, got %d", MaxIn, len(args))
	}
	if err := CheckInLimit("ids", 1001); err == nil {
		t.Fatal("expected error above the cap")
	}
	if err := CheckInLimit("ids", 1000); err != nil {
		t.Fatal("1000 must pass")
	}
}

func TestDateRangeHalfOpen(t *testing.T) {
	sql, args := New().DateRange("order_date", "2026-01-05", "2026-01-31").SQL()
	if sql != "order_date >= ? AND order_date < ?" {
		t.Fatalf("got %q", sql)
	}
	if args[0] != "2026-01-05 00:00:00" || args[1] != "2026-02-01 00:00:00" {
		t.Fatalf("args %v", args)
	}
	sql, args = New().DateRange("d", "garbage", "").SQL()
	if sql != "1=1" || len(args) != 0 {
		t.Fatalf("bad date must be ignored here: %q %v", sql, args)
	}
}

func TestDateRangeWIB(t *testing.T) {
	_, args := New().DateRangeWIB("created_at", "2026-01-05", "2026-01-05").SQL()
	if args[0] != "2026-01-04 17:00:00" || args[1] != "2026-01-05 17:00:00" {
		t.Fatalf("args %v", args)
	}
}

func TestParseCommon(t *testing.T) {
	spec := &SortSpec{Allowed: map[string]string{"date": "order_date", "number": "number"}, Default: "date", DefaultDesc: true}
	c, err := ParseCommon(url.Values{"search": {" budi "}, "status": {"a,b"}, "from": {"2026-01-01"},
		"to": {"2026-01-31"}, "partyId": {"9"}, "sort": {"number"}, "dir": {"asc"}, "page": {"2"}, "limit": {"10"}}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if c.Search != "budi" || len(c.Statuses) != 2 || c.PartyID != 9 || c.Sort != "number" || c.Desc || c.Page != 2 || c.Limit != 10 || c.Offset() != 10 {
		t.Fatalf("unexpected %+v", c)
	}
	if got := c.OrderBy("id DESC"); got != "number ASC" {
		t.Fatalf("got %q", got)
	}
	c, err = ParseCommon(url.Values{}, spec)
	if err != nil || c.Sort != "order_date" || !c.Desc {
		t.Fatalf("defaults wrong: %+v %v", c, err)
	}
}

func TestParseCommonRejects(t *testing.T) {
	spec := &SortSpec{Allowed: map[string]string{"date": "order_date"}, Default: "date"}
	cases := map[string]url.Values{
		"from":    {"from": {"2026-13-01"}},
		"to":      {"to": {"31/01/2026"}},
		"partyId": {"partyId": {"abc"}},
		"sort":    {"sort": {"id; DROP TABLE x"}},
		"dir":     {"dir": {"sideways"}},
	}
	for field, q := range cases {
		_, err := ParseCommon(q, spec)
		ae, ok := err.(*apperror.AppError)
		if !ok || len(ae.Fields) == 0 || ae.Fields[0].Field != field {
			t.Fatalf("%s: got %v", field, err)
		}
	}
	if _, err := ParseCommon(url.Values{"from": {"2026-02-01"}, "to": {"2026-01-01"}}, nil); err == nil {
		t.Fatal("from after to must fail")
	}
}

func TestValidateEnum(t *testing.T) {
	if ValidateEnum("paymentTerms", "", "cod", "net") != nil || ValidateEnum("paymentTerms", "net", "cod", "net") != nil {
		t.Fatal("valid values rejected")
	}
	if ValidateEnum("paymentTerms", "x", "cod", "net") == nil {
		t.Fatal("invalid accepted")
	}
}
