package database

import (
	"fmt"
	"strings"
	"time"
)

type Dialect interface {
	Placeholder(n int) string
	ParseTime(v interface{}) (time.Time, error)
	UpsertSQL(table string, columns string, conflictCols string) string
	Boolean(v bool) string
}

type PostgreSQLDialect struct{}

func (d *PostgreSQLDialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (d *PostgreSQLDialect) ParseTime(v interface{}) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case *time.Time:
		if t != nil {
			return *t, nil
		}
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("PostgreSQLDialect: unexpected time type %T", v)
	}
}

func (d *PostgreSQLDialect) UpsertSQL(table string, columns string, conflictCols string) string {
	colList := splitCSV(columns)
	conflictList := splitCSV(conflictCols)

	var placeholders []string
	for i := 1; i <= len(colList); i++ {
		placeholders = append(placeholders, d.Placeholder(i))
	}

	var updateClauses []string
	conflictSet := toSet(conflictList)
	for _, col := range colList {
		if conflictSet[col] {
			continue
		}
		updateClauses = append(updateClauses, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
		table, columns, strings.Join(placeholders, ", "), conflictCols, strings.Join(updateClauses, ", "))
}

func (d *PostgreSQLDialect) Boolean(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

type SQLiteDialect struct{}

func (d *SQLiteDialect) Placeholder(n int) string {
	return "?"
}

func (d *SQLiteDialect) ParseTime(v interface{}) (time.Time, error) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return time.Time{}, nil
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", t, time.Local); err == nil {
			return parsed, nil
		}
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return parsed, nil
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed, nil
		}
		return time.Time{}, fmt.Errorf("SQLiteDialect: unable to parse time %q", t)
	case time.Time:
		return t, nil
	default:
		return time.Time{}, fmt.Errorf("SQLiteDialect: unexpected time type %T", v)
	}
}

func (d *SQLiteDialect) UpsertSQL(table string, columns string, conflictCols string) string {
	colList := splitCSV(columns)
	conflictList := splitCSV(conflictCols)

	placeholders := make([]string, len(colList))
	for i := range colList {
		placeholders[i] = "?"
	}

	var updateClauses []string
	conflictSet := toSet(conflictList)
	for _, col := range colList {
		if conflictSet[col] {
			continue
		}
		updateClauses = append(updateClauses, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
		table, columns, strings.Join(placeholders, ", "), conflictCols, strings.Join(updateClauses, ", "))
}

func (d *SQLiteDialect) Boolean(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
