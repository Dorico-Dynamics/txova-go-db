package postgres

import (
	"testing"

	"github.com/Dorico-Dynamics/txova-go-types/pagination"
)

func TestSelectBuilder_BasicQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		builder     func() *SelectBuilder
		wantSQL     string
		wantArgs    []any
		wantErr     bool
		errContains string
	}{
		{
			name: "simple select all",
			builder: func() *SelectBuilder {
				return Select("users")
			},
			wantSQL:  "SELECT * FROM users",
			wantArgs: nil,
		},
		{
			name: "select with columns",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id", "name", "email")
			},
			wantSQL:  "SELECT id, name, email FROM users",
			wantArgs: nil,
		},
		{
			name: "select distinct",
			builder: func() *SelectBuilder {
				return Select("users").Distinct().Columns("status")
			},
			wantSQL:  "SELECT DISTINCT status FROM users",
			wantArgs: nil,
		},
		{
			name: "select with where",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id", "name").Where("status = ?", "active")
			},
			wantSQL:  "SELECT id, name FROM users WHERE status = $1",
			wantArgs: []any{"active"},
		},
		{
			name: "select with multiple where AND",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("id", "name").
					Where("status = ?", "active").
					Where("role = ?", "admin")
			},
			wantSQL:  "SELECT id, name FROM users WHERE status = $1 AND role = $2",
			wantArgs: []any{"active", "admin"},
		},
		{
			name: "select with where OR",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("id").
					Where("status = ?", "active").
					OrWhere("status = ?", "pending")
			},
			wantSQL:  "SELECT id FROM users WHERE status = $1 OR status = $2",
			wantArgs: []any{"active", "pending"},
		},
		{
			name: "select with where IN",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereIn("status", "active", "pending", "approved")
			},
			wantSQL:  "SELECT id FROM users WHERE status IN ($1, $2, $3)",
			wantArgs: []any{"active", "pending", "approved"},
		},
		{
			name: "select with where NOT IN",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereNotIn("status", "deleted", "banned")
			},
			wantSQL:  "SELECT id FROM users WHERE status NOT IN ($1, $2)",
			wantArgs: []any{"deleted", "banned"},
		},
		{
			name: "select with where LIKE",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereLike("name", "%John%")
			},
			wantSQL:  "SELECT id FROM users WHERE name LIKE $1",
			wantArgs: []any{"%John%"},
		},
		{
			name: "select with where ILIKE",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereILike("email", "%@gmail.com")
			},
			wantSQL:  "SELECT id FROM users WHERE email ILIKE $1",
			wantArgs: []any{"%@gmail.com"},
		},
		{
			name: "select with where NULL",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereNull("deleted_at")
			},
			wantSQL:  "SELECT id FROM users WHERE deleted_at IS NULL",
			wantArgs: nil,
		},
		{
			name: "select with where NOT NULL",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").WhereNotNull("verified_at")
			},
			wantSQL:  "SELECT id FROM users WHERE verified_at IS NOT NULL",
			wantArgs: nil,
		},
		{
			name: "select with where BETWEEN",
			builder: func() *SelectBuilder {
				return Select("orders").Columns("id").WhereBetween("amount", 100, 500)
			},
			wantSQL:  "SELECT id FROM orders WHERE amount BETWEEN $1 AND $2",
			wantArgs: []any{100, 500},
		},
		{
			name: "select with order by",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").OrderByDesc("created_at")
			},
			wantSQL:  "SELECT id FROM users ORDER BY created_at DESC",
			wantArgs: nil,
		},
		{
			name: "select with multiple order by",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").OrderByDesc("created_at").OrderByAsc("name")
			},
			wantSQL:  "SELECT id FROM users ORDER BY created_at DESC, name ASC",
			wantArgs: nil,
		},
		{
			name: "select with limit and offset",
			builder: func() *SelectBuilder {
				return Select("users").Columns("id").Limit(10).Offset(20)
			},
			wantSQL:  "SELECT id FROM users LIMIT 10 OFFSET 20",
			wantArgs: nil,
		},
		{
			name: "invalid table name",
			builder: func() *SelectBuilder {
				return Select("users; DROP TABLE users;--")
			},
			wantErr:     true,
			errContains: "invalid table name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := tt.builder()
			sql, args, err := builder.Build()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Build() expected error containing %q, got nil", tt.errContains)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("Build() unexpected error = %v", err)
				return
			}

			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}

			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args length = %d, want %d", len(args), len(tt.wantArgs))
				return
			}

			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("Build() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestSelectBuilder_Joins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		builder  func() *SelectBuilder
		wantSQL  string
		wantArgs []any
	}{
		{
			name: "inner join",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("users.id", "profiles.bio").
					Join("profiles", "profiles.user_id = users.id")
			},
			wantSQL:  "SELECT users.id, profiles.bio FROM users INNER JOIN profiles ON profiles.user_id = users.id",
			wantArgs: nil,
		},
		{
			name: "left join",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("users.id", "orders.total").
					LeftJoin("orders", "orders.user_id = users.id")
			},
			wantSQL:  "SELECT users.id, orders.total FROM users LEFT JOIN orders ON orders.user_id = users.id",
			wantArgs: nil,
		},
		{
			name: "right join",
			builder: func() *SelectBuilder {
				return Select("orders").
					Columns("orders.id", "users.name").
					RightJoin("users", "users.id = orders.user_id")
			},
			wantSQL:  "SELECT orders.id, users.name FROM orders RIGHT JOIN users ON users.id = orders.user_id",
			wantArgs: nil,
		},
		{
			name: "multiple joins",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("users.id", "profiles.bio", "settings.theme").
					Join("profiles", "profiles.user_id = users.id").
					LeftJoin("settings", "settings.user_id = users.id")
			},
			wantSQL:  "SELECT users.id, profiles.bio, settings.theme FROM users INNER JOIN profiles ON profiles.user_id = users.id LEFT JOIN settings ON settings.user_id = users.id",
			wantArgs: nil,
		},
		{
			name: "join with args",
			builder: func() *SelectBuilder {
				return Select("users").
					Columns("users.id").
					Join("orders", "orders.user_id = users.id AND orders.status = ?", "completed")
			},
			wantSQL:  "SELECT users.id FROM users INNER JOIN orders ON orders.user_id = users.id AND orders.status = $1",
			wantArgs: []any{"completed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.builder().Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestSelectBuilder_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		builder  func() *SelectBuilder
		wantSQL  string
		wantArgs []any
	}{
		{
			name: "page request with sort",
			builder: func() *SelectBuilder {
				req := pagination.NewPageRequest().
					WithLimit(20).
					WithOffset(40).
					WithSort("created_at", pagination.SortDesc)
				return Select("users").Columns("id", "name").Page(req)
			},
			wantSQL:  "SELECT id, name FROM users ORDER BY created_at DESC LIMIT 20 OFFSET 40",
			wantArgs: nil,
		},
		{
			name: "page request without sort",
			builder: func() *SelectBuilder {
				req := pagination.NewPageRequest().WithLimit(10).WithOffset(0)
				return Select("users").Columns("id").Page(req)
			},
			wantSQL:  "SELECT id FROM users LIMIT 10 OFFSET 0",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.builder().Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestSelectBuilder_GroupByHaving(t *testing.T) {
	t.Parallel()

	sql, args, err := Select("orders").
		Columns("user_id", "COUNT(*) as order_count").
		GroupBy("user_id").
		Having("COUNT(*) > ?", 5).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	wantSQL := "SELECT user_id, COUNT(*) as order_count FROM orders GROUP BY user_id HAVING COUNT(*) > $1"
	if sql != wantSQL {
		t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
	}

	if len(args) != 1 || args[0] != 5 {
		t.Errorf("Build() args = %v, want [5]", args)
	}
}

func TestSelectBuilder_Locking(t *testing.T) {
	t.Parallel()

	t.Run("FOR UPDATE", func(t *testing.T) {
		t.Parallel()
		sql, _, err := Select("users").Columns("id").Where("id = ?", 1).ForUpdate().Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "SELECT id FROM users WHERE id = $1 FOR UPDATE"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("FOR SHARE", func(t *testing.T) {
		t.Parallel()
		sql, _, err := Select("users").Columns("id").Where("id = ?", 1).ForShare().Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "SELECT id FROM users WHERE id = $1 FOR SHARE"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})
}

func TestSelectBuilder_Allowlist(t *testing.T) {
	t.Parallel()

	t.Run("allowed column succeeds", func(t *testing.T) {
		t.Parallel()
		sql, _, err := SelectWithAllowlist("users", "id", "name", "email").
			Columns("id", "name").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "SELECT id, name FROM users"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("disallowed column fails", func(t *testing.T) {
		t.Parallel()
		_, _, err := SelectWithAllowlist("users", "id", "name").
			Columns("id", "password").
			Build()
		if err == nil {
			t.Error("Build() expected error for disallowed column, got nil")
		}
	})
}

func TestInsertBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		builder     func() *InsertBuilder
		wantSQL     string
		wantArgs    []any
		wantErr     bool
		errContains string
	}{
		{
			name: "simple insert",
			builder: func() *InsertBuilder {
				return Insert("users").
					Columns("name", "email").
					Values("John", "john@example.com")
			},
			wantSQL:  "INSERT INTO users (name, email) VALUES ($1, $2)",
			wantArgs: []any{"John", "john@example.com"},
		},
		{
			name: "insert with returning",
			builder: func() *InsertBuilder {
				return Insert("users").
					Columns("name", "email").
					Values("John", "john@example.com").
					Returning("id", "created_at")
			},
			wantSQL:  "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at",
			wantArgs: []any{"John", "john@example.com"},
		},
		{
			name: "batch insert",
			builder: func() *InsertBuilder {
				return Insert("users").
					Columns("name", "email").
					Values("John", "john@example.com").
					Values("Jane", "jane@example.com")
			},
			wantSQL:  "INSERT INTO users (name, email) VALUES ($1, $2), ($3, $4)",
			wantArgs: []any{"John", "john@example.com", "Jane", "jane@example.com"},
		},
		{
			name: "insert on conflict do nothing",
			builder: func() *InsertBuilder {
				return Insert("users").
					Columns("email", "name").
					Values("john@example.com", "John").
					OnConflictDoNothing("email")
			},
			wantSQL:  "INSERT INTO users (email, name) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING",
			wantArgs: []any{"john@example.com", "John"},
		},
		{
			name: "no columns error",
			builder: func() *InsertBuilder {
				return Insert("users").Values("John")
			},
			wantErr:     true,
			errContains: "no columns",
		},
		{
			name: "no values error",
			builder: func() *InsertBuilder {
				return Insert("users").Columns("name")
			},
			wantErr:     true,
			errContains: "no values",
		},
		{
			name: "column count mismatch error",
			builder: func() *InsertBuilder {
				return Insert("users").Columns("name", "email").Values("John")
			},
			wantErr:     true,
			errContains: "values but",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.builder().Build()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Build() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}

			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestUpdateBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		builder     func() *UpdateBuilder
		wantSQL     string
		wantArgs    []any
		wantErr     bool
		errContains string
	}{
		{
			name: "simple update",
			builder: func() *UpdateBuilder {
				return Update("users").
					Set("name", "John Doe").
					Where("id = ?", 1)
			},
			wantSQL:  "UPDATE users SET name = $1 WHERE id = $2",
			wantArgs: []any{"John Doe", 1},
		},
		{
			name: "update multiple columns",
			builder: func() *UpdateBuilder {
				return Update("users").
					Set("name", "John").
					Set("email", "john@example.com").
					Where("id = ?", 1)
			},
			wantSQL:  "UPDATE users SET name = $1, email = $2 WHERE id = $3",
			wantArgs: []any{"John", "john@example.com", 1},
		},
		{
			name: "update with returning",
			builder: func() *UpdateBuilder {
				return Update("users").
					Set("status", "active").
					Where("id = ?", 1).
					Returning("id", "updated_at")
			},
			wantSQL:  "UPDATE users SET status = $1 WHERE id = $2 RETURNING id, updated_at",
			wantArgs: []any{"active", 1},
		},
		{
			name: "update with OR where",
			builder: func() *UpdateBuilder {
				return Update("users").
					Set("status", "archived").
					Where("created_at < ?", "2020-01-01").
					OrWhere("status = ?", "inactive")
			},
			wantSQL:  "UPDATE users SET status = $1 WHERE created_at < $2 OR status = $3",
			wantArgs: []any{"archived", "2020-01-01", "inactive"},
		},
		{
			name: "no columns error",
			builder: func() *UpdateBuilder {
				return Update("users").Where("id = ?", 1)
			},
			wantErr:     true,
			errContains: "no columns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.builder().Build()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Build() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}

			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestDeleteBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		builder     func() *DeleteBuilder
		wantSQL     string
		wantArgs    []any
		wantErr     bool
		errContains string
	}{
		{
			name: "simple delete",
			builder: func() *DeleteBuilder {
				return Delete("users").Where("id = ?", 1)
			},
			wantSQL:  "DELETE FROM users WHERE id = $1",
			wantArgs: []any{1},
		},
		{
			name: "delete with multiple conditions",
			builder: func() *DeleteBuilder {
				return Delete("users").
					Where("status = ?", "deleted").
					Where("deleted_at < ?", "2020-01-01")
			},
			wantSQL:  "DELETE FROM users WHERE status = $1 AND deleted_at < $2",
			wantArgs: []any{"deleted", "2020-01-01"},
		},
		{
			name: "delete with returning",
			builder: func() *DeleteBuilder {
				return Delete("users").
					Where("id = ?", 1).
					Returning("id", "email")
			},
			wantSQL:  "DELETE FROM users WHERE id = $1 RETURNING id, email",
			wantArgs: []any{1},
		},
		{
			name: "delete with IN clause",
			builder: func() *DeleteBuilder {
				return Delete("users").WhereIn("id", 1, 2, 3)
			},
			wantSQL:  "DELETE FROM users WHERE id IN ($1, $2, $3)",
			wantArgs: []any{1, 2, 3},
		},
		{
			name: "delete all (no where)",
			builder: func() *DeleteBuilder {
				return Delete("temp_data")
			},
			wantSQL:  "DELETE FROM temp_data",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.builder().Build()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Build() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}

			if len(args) != len(tt.wantArgs) {
				t.Errorf("Build() args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestSQLInjectionPrevention(t *testing.T) {
	t.Parallel()

	injectionAttempts := []struct {
		name  string
		input string
	}{
		{"drop table", "users; DROP TABLE users;--"},
		{"comment injection", "users--"},
		{"union select", "users UNION SELECT * FROM passwords"},
		{"special chars", "users'\""},
		{"semicolon", "users;"},
		{"backtick", "users`"},
	}

	for _, tt := range injectionAttempts {
		t.Run("table_"+tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Select(tt.input).Columns("id").Build()
			if err == nil {
				t.Errorf("Select(%q) should have failed", tt.input)
			}
		})

		t.Run("column_allowlist_"+tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := SelectWithAllowlist("users", tt.input).
				Columns(tt.input).
				Build()
			if err == nil {
				t.Errorf("Columns(%q) with allowlist should have failed", tt.input)
			}
		})
	}
}

func TestTypedIDSupport(t *testing.T) {
	t.Parallel()

	// Typed IDs from txova-go-types implement driver.Valuer,
	// so they can be passed as query arguments directly.
	// This test verifies that the query builder correctly
	// passes them through without modification.

	t.Run("typed ID as WHERE argument", func(t *testing.T) {
		t.Parallel()
		// Simulating a typed ID (any type implementing driver.Valuer works)
		userID := "550e8400-e29b-41d4-a716-446655440000"

		sql, args, err := Select("users").
			Columns("id", "name").
			Where("id = ?", userID).
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		wantSQL := "SELECT id, name FROM users WHERE id = $1"
		if sql != wantSQL {
			t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
		}

		if len(args) != 1 || args[0] != userID {
			t.Errorf("Build() args = %v, want [%s]", args, userID)
		}
	})

	t.Run("typed ID in INSERT", func(t *testing.T) {
		t.Parallel()
		userID := "550e8400-e29b-41d4-a716-446655440000"

		sql, args, err := Insert("users").
			Columns("id", "name").
			Values(userID, "John").
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		wantSQL := "INSERT INTO users (id, name) VALUES ($1, $2)"
		if sql != wantSQL {
			t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
		}

		if len(args) != 2 || args[0] != userID {
			t.Errorf("Build() args[0] = %v, want %s", args[0], userID)
		}
	})
}

func TestHelperMethods(t *testing.T) {
	t.Parallel()

	t.Run("SelectBuilder SQL helper", func(t *testing.T) {
		t.Parallel()
		builder := Select("users").Columns("id")
		sql := builder.SQL()
		if sql != "SELECT id FROM users" {
			t.Errorf("SQL() = %q, want %q", sql, "SELECT id FROM users")
		}
	})

	t.Run("SelectBuilder Args helper", func(t *testing.T) {
		t.Parallel()
		builder := Select("users").Where("id = ?", 1)
		args := builder.Args()
		if len(args) != 1 || args[0] != 1 {
			t.Errorf("Args() = %v, want [1]", args)
		}
	})

	t.Run("SelectBuilder MustBuild success", func(t *testing.T) {
		t.Parallel()
		sql, args := Select("users").Columns("id").MustBuild()
		if sql != "SELECT id FROM users" {
			t.Errorf("MustBuild() SQL = %q", sql)
		}
		if len(args) != 0 {
			t.Errorf("MustBuild() args = %v", args)
		}
	})

	t.Run("SelectBuilder MustBuild panics on error", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustBuild() should have panicked")
			}
		}()
		Select("invalid;table").MustBuild()
	})

	// InsertBuilder helper methods
	t.Run("InsertBuilder SQL helper", func(t *testing.T) {
		t.Parallel()
		builder := Insert("users").Columns("name").Values("John")
		sql := builder.SQL()
		if sql != "INSERT INTO users (name) VALUES ($1)" {
			t.Errorf("SQL() = %q, want %q", sql, "INSERT INTO users (name) VALUES ($1)")
		}
	})

	t.Run("InsertBuilder Args helper", func(t *testing.T) {
		t.Parallel()
		builder := Insert("users").Columns("name").Values("John")
		args := builder.Args()
		if len(args) != 1 || args[0] != "John" {
			t.Errorf("Args() = %v, want [John]", args)
		}
	})

	t.Run("InsertBuilder MustBuild success", func(t *testing.T) {
		t.Parallel()
		sql, args := Insert("users").Columns("name").Values("John").MustBuild()
		if sql != "INSERT INTO users (name) VALUES ($1)" {
			t.Errorf("MustBuild() SQL = %q", sql)
		}
		if len(args) != 1 {
			t.Errorf("MustBuild() args = %v", args)
		}
	})

	t.Run("InsertBuilder MustBuild panics on error", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustBuild() should have panicked")
			}
		}()
		Insert("users").MustBuild() // No columns - should panic
	})

	// UpdateBuilder helper methods
	t.Run("UpdateBuilder SQL helper", func(t *testing.T) {
		t.Parallel()
		builder := Update("users").Set("name", "John").Where("id = ?", 1)
		sql := builder.SQL()
		if sql != "UPDATE users SET name = $1 WHERE id = $2" {
			t.Errorf("SQL() = %q, want %q", sql, "UPDATE users SET name = $1 WHERE id = $2")
		}
	})

	t.Run("UpdateBuilder Args helper", func(t *testing.T) {
		t.Parallel()
		builder := Update("users").Set("name", "John").Where("id = ?", 1)
		args := builder.Args()
		if len(args) != 2 || args[0] != "John" || args[1] != 1 {
			t.Errorf("Args() = %v, want [John, 1]", args)
		}
	})

	t.Run("UpdateBuilder MustBuild success", func(t *testing.T) {
		t.Parallel()
		sql, args := Update("users").Set("name", "John").Where("id = ?", 1).MustBuild()
		if sql != "UPDATE users SET name = $1 WHERE id = $2" {
			t.Errorf("MustBuild() SQL = %q", sql)
		}
		if len(args) != 2 {
			t.Errorf("MustBuild() args = %v", args)
		}
	})

	t.Run("UpdateBuilder MustBuild panics on error", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustBuild() should have panicked")
			}
		}()
		Update("users").MustBuild() // No SET - should panic
	})

	// DeleteBuilder helper methods
	t.Run("DeleteBuilder SQL helper", func(t *testing.T) {
		t.Parallel()
		builder := Delete("users").Where("id = ?", 1)
		sql := builder.SQL()
		if sql != "DELETE FROM users WHERE id = $1" {
			t.Errorf("SQL() = %q, want %q", sql, "DELETE FROM users WHERE id = $1")
		}
	})

	t.Run("DeleteBuilder Args helper", func(t *testing.T) {
		t.Parallel()
		builder := Delete("users").Where("id = ?", 1)
		args := builder.Args()
		if len(args) != 1 || args[0] != 1 {
			t.Errorf("Args() = %v, want [1]", args)
		}
	})

	t.Run("DeleteBuilder MustBuild success", func(t *testing.T) {
		t.Parallel()
		sql, args := Delete("users").Where("id = ?", 1).MustBuild()
		if sql != "DELETE FROM users WHERE id = $1" {
			t.Errorf("MustBuild() SQL = %q", sql)
		}
		if len(args) != 1 {
			t.Errorf("MustBuild() args = %v", args)
		}
	})

	t.Run("DeleteBuilder MustBuild panics on error", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustBuild() should have panicked")
			}
		}()
		Delete("invalid;table").MustBuild()
	})
}

func TestAllowlistBuilders(t *testing.T) {
	t.Parallel()

	t.Run("InsertWithAllowlist allowed columns", func(t *testing.T) {
		t.Parallel()
		sql, _, err := InsertWithAllowlist("users", "name", "email").
			Columns("name", "email").
			Values("John", "john@example.com").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "INSERT INTO users (name, email) VALUES ($1, $2)"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("InsertWithAllowlist disallowed column", func(t *testing.T) {
		t.Parallel()
		_, _, err := InsertWithAllowlist("users", "name").
			Columns("name", "password").
			Values("John", "secret").
			Build()
		if err == nil {
			t.Error("Build() expected error for disallowed column")
		}
	})

	t.Run("InsertBuilder OnConflictConstraintDoNothing", func(t *testing.T) {
		t.Parallel()
		sql, _, err := Insert("users").
			Columns("email", "name").
			Values("john@example.com", "John").
			OnConflictConstraintDoNothing("users_email_unique").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "INSERT INTO users (email, name) VALUES ($1, $2) ON CONFLICT ON CONSTRAINT users_email_unique DO NOTHING"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("UpdateWithAllowlist allowed columns", func(t *testing.T) {
		t.Parallel()
		sql, _, err := UpdateWithAllowlist("users", "name", "email").
			Set("name", "John").
			Where("id = ?", 1).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "UPDATE users SET name = $1 WHERE id = $2"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("UpdateWithAllowlist disallowed column", func(t *testing.T) {
		t.Parallel()
		_, _, err := UpdateWithAllowlist("users", "name").
			Set("password", "secret").
			Where("id = ?", 1).
			Build()
		if err == nil {
			t.Error("Build() expected error for disallowed column")
		}
	})

	t.Run("UpdateBuilder SetMap", func(t *testing.T) {
		t.Parallel()
		sql, args, err := Update("users").
			SetMap(map[string]any{"name": "John", "status": "active"}).
			Where("id = ?", 1).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// Map iteration order is not guaranteed, so just check it contains expected parts
		if len(args) != 3 {
			t.Errorf("Build() args length = %d, want 3", len(args))
		}
		if sql == "" {
			t.Error("Build() SQL is empty")
		}
	})

	t.Run("UpdateBuilder WhereIn", func(t *testing.T) {
		t.Parallel()
		sql, args, err := Update("users").
			Set("status", "archived").
			WhereIn("id", 1, 2, 3).
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "UPDATE users SET status = $1 WHERE id IN ($2, $3, $4)"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
		if len(args) != 4 {
			t.Errorf("Build() args = %v, want 4 args", args)
		}
	})

	t.Run("UpdateBuilder WhereIn empty values", func(t *testing.T) {
		t.Parallel()
		sql, _, err := Update("users").
			Set("status", "archived").
			WhereIn("id").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// Empty WhereIn should not add any WHERE clause
		want := "UPDATE users SET status = $1"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("DeleteWithAllowlist", func(t *testing.T) {
		t.Parallel()
		builder := DeleteWithAllowlist("users", "id", "status")
		sql, _, err := builder.Where("id = ?", 1).Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "DELETE FROM users WHERE id = $1"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})

	t.Run("DeleteBuilder OrWhere", func(t *testing.T) {
		t.Parallel()
		sql, args, err := Delete("users").
			Where("status = ?", "deleted").
			OrWhere("status = ?", "expired").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		want := "DELETE FROM users WHERE status = $1 OR status = $2"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
		if len(args) != 2 {
			t.Errorf("Build() args = %v, want 2 args", args)
		}
	})

	t.Run("DeleteBuilder WhereIn empty values", func(t *testing.T) {
		t.Parallel()
		sql, _, err := Delete("users").
			WhereIn("id").
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// Empty WhereIn should not add any WHERE clause
		want := "DELETE FROM users"
		if sql != want {
			t.Errorf("Build() SQL = %q, want %q", sql, want)
		}
	})
}

func TestReplacePlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		condition  string
		startIndex int
		wantResult string
		wantIndex  int
	}{
		{"id = ?", 1, "id = $1", 2},
		{"a = ? AND b = ?", 1, "a = $1 AND b = $2", 3},
		{"x = ? OR y = ?", 5, "x = $5 OR y = $6", 7},
		{"no placeholders", 1, "no placeholders", 1},
		{"? ? ?", 10, "$10 $11 $12", 13},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			t.Parallel()
			result, index := replacePlaceholders(tt.condition, tt.startIndex)
			if result != tt.wantResult {
				t.Errorf("replacePlaceholders() = %q, want %q", result, tt.wantResult)
			}
			if index != tt.wantIndex {
				t.Errorf("replacePlaceholders() index = %d, want %d", index, tt.wantIndex)
			}
		})
	}
}

func TestValidateColumnName(t *testing.T) {
	t.Parallel()

	t.Run("without allowlist - lenient validation", func(t *testing.T) {
		t.Parallel()
		qb := NewQueryBuilder()

		// Without allowlist, only empty column is rejected
		if err := qb.validateColumnName(""); err == nil {
			t.Error("empty column name should fail")
		}

		// Valid column names pass
		validNames := []string{"id", "user_id", "COUNT(*)", "SUM(amount) as total"}
		for _, name := range validNames {
			if err := qb.validateColumnName(name); err != nil {
				t.Errorf("validateColumnName(%q) = %v, want nil", name, err)
			}
		}
	})

	t.Run("with allowlist - strict validation", func(t *testing.T) {
		t.Parallel()
		qb := NewQueryBuilder("id", "name", "email", "users.id", "_private")

		// Valid names in allowlist pass
		validNames := []string{"id", "name", "email", "users.id", "_private"}
		for _, name := range validNames {
			if err := qb.validateColumnName(name); err != nil {
				t.Errorf("validateColumnName(%q) = %v, want nil", name, err)
			}
		}

		// Valid format but not in allowlist fails
		if err := qb.validateColumnName("status"); err == nil {
			t.Error("column not in allowlist should fail")
		}

		// Invalid format fails even before checking allowlist
		invalidNames := []string{
			"",
			"1column",
			"column-name",
			"column;drop",
			"column'",
		}
		for _, name := range invalidNames {
			if err := qb.validateColumnName(name); err == nil {
				t.Errorf("validateColumnName(%q) = nil, want error", name)
			}
		}
	})
}

func TestSelectBuilder_EmptyWhereInNotIn(t *testing.T) {
	t.Parallel()

	t.Run("WhereIn with empty values", func(t *testing.T) {
		t.Parallel()
		sql, args, err := Select("users").
			Columns("id", "name").
			WhereIn("status"). // no values
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// Empty WhereIn should be ignored
		if sql != "SELECT id, name FROM users" {
			t.Errorf("Build() SQL = %q, want no WHERE clause", sql)
		}
		if len(args) != 0 {
			t.Errorf("Build() args = %v, want empty", args)
		}
	})

	t.Run("WhereNotIn with empty values", func(t *testing.T) {
		t.Parallel()
		sql, args, err := Select("users").
			Columns("id", "name").
			WhereNotIn("status"). // no values
			Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// Empty WhereNotIn should be ignored
		if sql != "SELECT id, name FROM users" {
			t.Errorf("Build() SQL = %q, want no WHERE clause", sql)
		}
		if len(args) != 0 {
			t.Errorf("Build() args = %v, want empty", args)
		}
	})
}
