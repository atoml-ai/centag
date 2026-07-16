package tokenusage

import "regexp"

var pgPlaceholderRe = regexp.MustCompile(`\$\d+`)

// q adapts PostgreSQL $N placeholders to SQLite ? when needed.
func (s *Service) q(sql string) string {
	if s.isPostgres() {
		return sql
	}
	return pgPlaceholderRe.ReplaceAllString(sql, "?")
}