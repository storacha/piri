package dialect

import (
	"strconv"
	"strings"
)

// Dialect represents the SQL database dialect being used.
type Dialect string

const (
	// SQLite is the SQLite dialect (default).
	SQLite Dialect = "sqlite"
	// Postgres is the PostgreSQL dialect.
	Postgres Dialect = "postgres"
)

// InsertIgnore returns the appropriate INSERT IGNORE/ON CONFLICT DO NOTHING syntax.
func (d Dialect) InsertIgnore(table, columns, placeholders string) string {
	switch d {
	case Postgres:
		return "INSERT INTO " + table + "(" + columns + ") VALUES(" + d.Rebind(placeholders) + ") ON CONFLICT DO NOTHING"
	default: // SQLite
		return "INSERT OR IGNORE INTO " + table + "(" + columns + ") VALUES(" + placeholders + ")"
	}
}

// Rebind converts ? placeholders to the appropriate dialect format.
// For PostgreSQL, ? is converted to $1, $2, etc.
// For SQLite, the query is returned unchanged.
func (d Dialect) Rebind(query string) string {
	if d != Postgres {
		return query
	}

	var buf strings.Builder
	buf.Grow(len(query) + 10)

	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			buf.WriteByte('$')
			buf.WriteString(strconv.Itoa(n))
			n++
		} else {
			buf.WriteByte(query[i])
		}
	}

	return buf.String()
}

// IsPostgres returns true if the dialect is PostgreSQL.
func (d Dialect) IsPostgres() bool {
	return d == Postgres
}

// IsSQLite returns true if the dialect is SQLite (or empty/default).
func (d Dialect) IsSQLite() bool {
	return d == "" || d == SQLite
}
