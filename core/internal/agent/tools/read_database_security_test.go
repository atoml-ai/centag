package tools

import "testing"

// R-P1：自由 query 必须只读且引用表均在白名单内，防止绕过表白名单读取任意表。
func TestValidateReadOnlyQuery(t *testing.T) {
	allowed := []string{"agent_sessions", "system_config", "token_usage"}

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"select allowed", "SELECT * FROM agent_sessions LIMIT 10", false},
		{"select lowercase", "select id, payload from system_config", false},
		{"cte select", "WITH t AS (SELECT * FROM token_usage) SELECT * FROM t", false},
		{"join allowed", "SELECT a.id FROM agent_sessions a JOIN system_config s ON a.id = s.id", false},
		{"comma from list", "SELECT * FROM agent_sessions, system_config", false},
		{"quoted ident", `SELECT * FROM "system_config"`, false},
		{"backtick ident", "SELECT * FROM `token_usage`", false},
		{"bracket ident", "SELECT * FROM [token_usage]", false},

		{"update denied", "UPDATE users SET name = 'x'", true},
		{"delete denied", "DELETE FROM users", true},
		{"insert denied", "INSERT INTO agent_sessions VALUES (1)", true},
		{"drop denied", "DROP TABLE users", true},
		{"cte write denied", "WITH d AS (DELETE FROM users RETURNING *) SELECT * FROM d", true},
		{"multi statement denied", "SELECT 1; DELETE FROM users", true},
		{"pragma denied", "PRAGMA table_info(users)", true},
		{"non-select prefix denied", "EXPLAIN SELECT * FROM users", true},
		{"empty denied", "   ", true},

		// 表白名单绕过类
		{"undeclared table denied", "SELECT * FROM users", true},
		{"undeclared via join denied", "SELECT * FROM agent_sessions u JOIN api_keys k ON 1=1", true},
		{"undeclared via comma denied", "SELECT * FROM system_config, api_keys", true},
		{"schema qualified undeclared denied", "SELECT * FROM public.users", true},
		// 块注释内容被剥离：注释里的表引用不参与校验（也不放行真实引用）
		{"comment hidden table ignored", "SELECT * /* from users */ FROM agent_sessions", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadOnlyQuery(tt.query, allowed)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateReadOnlyQuery(%q) err = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

func TestExtractQueryTables(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"SELECT * FROM users", []string{"users"}},
		{"select a from t1 join t2 on a.id=t2.id", []string{"t1", "t2"}},
		{"SELECT * FROM t1, t2, t3", []string{"t1", "t2", "t3"}},
		{`SELECT * FROM "Users" u`, []string{"Users"}},
		{"SELECT * FROM public.users u JOIN billing.orders o ON 1=1", []string{"users", "orders"}},
		{"with x as (select * from logs) select * from x, meta", []string{"logs", "x", "meta"}},
	}
	for _, c := range cases {
		got := extractQueryTables(c.query)
		if len(got) != len(c.want) {
			t.Fatalf("extractQueryTables(%q) = %v, want %v", c.query, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("extractQueryTables(%q) = %v, want %v", c.query, got, c.want)
			}
		}
	}
}

func TestExtractCTENames(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"WITH t AS (SELECT 1) SELECT * FROM t", []string{"t"}},
		{"WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b", []string{"a", "b"}},
		{"SELECT * FROM plain_table", nil},
	}
	for _, c := range cases {
		got := extractCTENames(c.query)
		if len(got) != len(c.want) {
			t.Fatalf("extractCTENames(%q) = %v, want %v", c.query, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("extractCTENames(%q) = %v, want %v", c.query, got, c.want)
			}
		}
	}
}

func TestStripSQLComments(t *testing.T) {
	in := "SELECT * -- comment drop\nFROM t /* delete */ WHERE id = 1"
	out := stripSQLComments(in)
	if containsAny(out, "--", "/*") {
		t.Fatalf("comments not stripped: %q", out)
	}
	for _, want := range []string{"SELECT * ", "\nFROM t ", " WHERE id = 1"} {
		if !containsSub(out, want) {
			t.Fatalf("stripSQLComments output missing %q: %q", want, out)
		}
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if containsSub(s, sub) {
			return true
		}
	}
	return false
}

func containsSub(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
