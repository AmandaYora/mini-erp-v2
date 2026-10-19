package dsnguard

import "testing"

func TestCheck(t *testing.T) {
	cases := []struct {
		name        string
		dsn         string
		allowRemote bool
		wantErr     bool
	}{
		{"test db lokal", "root:@tcp(localhost:3306)/mini_erp_test", false, false},
		{"test db dengan query", "u:p@tcp(127.0.0.1:3306)/x_test?parseTime=true", false, false},
		{"ipv6 loopback", "u:p@tcp([::1]:3306)/x_test", false, false},
		{"database produksi", "root:@tcp(localhost:3306)/mini_erp_db", false, true},
		{"nama menyerupai", "root:@tcp(localhost:3306)/test_mini_erp", false, true},
		{"host jauh", "root@tcp(10.0.0.5:3306)/x_test", false, true},
		{"host jauh diizinkan CI", "root@tcp(db:3306)/x_test", true, false},
		{"db produksi di host jauh tetap ditolak", "root@tcp(db:3306)/mini_erp_db", true, true},
		{"tanpa database", "root:@tcp(localhost:3306)/", false, true},
		{"unix socket dianggap non-lokal", "root@unix(/tmp/mysql.sock)/x_test", false, true},
		{"query tidak menipu akhiran", "u:p@tcp(localhost:3306)/mini_erp_db?x=_test", false, true},
	}
	for _, c := range cases {
		err := Check(c.dsn, c.allowRemote)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: Check(%q) err=%v, wantErr=%v", c.name, c.dsn, err, c.wantErr)
		}
	}
}

func TestSplitAndHost(t *testing.T) {
	server, name := Split("u:p@tcp(localhost:3306)/mini_erp_test?parseTime=true")
	if server != "u:p@tcp(localhost:3306)/" || name != "mini_erp_test" {
		t.Fatalf("Split = %q, %q", server, name)
	}
	if h := Host("u:p@tcp(LocalHost:3306)/x"); h != "localhost" {
		t.Fatalf("Host = %q", h)
	}
	if h := Host("u:p@tcp([::1]:3306)/x"); h != "[::1]" {
		t.Fatalf("Host ipv6 = %q", h)
	}
}

func TestCheckName(t *testing.T) {
	if CheckName("mini_erp_test") != nil {
		t.Fatal("mini_erp_test harus lolos")
	}
	for _, bad := range []string{"", "mini_erp_db", "elkasir_db"} {
		if CheckName(bad) == nil {
			t.Errorf("CheckName(%q) harus ditolak", bad)
		}
	}
}
