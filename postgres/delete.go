// Package postgres provides PostgreSQL database utilities.
package postgres

import (
	"fmt"
	"strings"
)

// DeleteBuilder builds DELETE queries.
type DeleteBuilder struct {
	*QueryBuilder
	table             string
	where             []whereClause
	returning         []string
	allowUnrestricted bool
}

// Delete creates a new DeleteBuilder for the specified table.
func Delete(table string) *DeleteBuilder {
	return &DeleteBuilder{
		QueryBuilder: NewQueryBuilder(),
		table:        table,
		where:        []whereClause{},
		returning:    []string{},
	}
}

// DeleteWithAllowlist creates a new DeleteBuilder with column validation.
func DeleteWithAllowlist(table string, allowedColumns ...string) *DeleteBuilder {
	return &DeleteBuilder{
		QueryBuilder: NewQueryBuilder(allowedColumns...),
		table:        table,
		where:        []whereClause{},
		returning:    []string{},
	}
}

// Where adds a WHERE condition with AND logic.
func (d *DeleteBuilder) Where(condition string, args ...any) *DeleteBuilder {
	d.where = append(d.where, whereClause{condition: condition, args: args, isOr: false})
	return d
}

// OrWhere adds a WHERE condition with OR logic.
func (d *DeleteBuilder) OrWhere(condition string, args ...any) *DeleteBuilder {
	d.where = append(d.where, whereClause{condition: condition, args: args, isOr: true})
	return d
}

// WhereIn adds a WHERE column IN (...) condition.
func (d *DeleteBuilder) WhereIn(column string, values ...any) *DeleteBuilder {
	if len(values) == 0 {
		return d
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	condition := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	d.where = append(d.where, whereClause{condition: condition, args: values, isOr: false})
	return d
}

// Returning specifies columns to return after delete.
func (d *DeleteBuilder) Returning(columns ...string) *DeleteBuilder {
	d.returning = append(d.returning, columns...)
	return d
}

// AllowUnrestrictedDelete enables DELETE without WHERE clause.
// By default, DeleteBuilder requires at least one WHERE condition to prevent
// accidental full-table deletes. Call this method to explicitly allow
// unrestricted deletes when that is the intended behavior.
func (d *DeleteBuilder) AllowUnrestrictedDelete() *DeleteBuilder {
	d.allowUnrestricted = true
	return d
}

// Build generates the SQL query and returns it with the arguments.
func (d *DeleteBuilder) Build() (string, []any, error) {
	// Validate table name
	if err := validateTableName(d.table); err != nil {
		return "", nil, err
	}

	// Safeguard against unrestricted deletes
	if len(d.where) == 0 && !d.allowUnrestricted {
		return "", nil, fmt.Errorf("DELETE without WHERE clause requires explicit opt-in via AllowUnrestrictedDelete()")
	}

	var args []any
	argIndex := 1

	var sb strings.Builder

	// DELETE FROM clause
	sb.WriteString("DELETE FROM ")
	sb.WriteString(d.table)

	// WHERE clause
	if len(d.where) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range d.where {
			if i > 0 {
				if w.isOr {
					sb.WriteString(" OR ")
				} else {
					sb.WriteString(" AND ")
				}
			}
			condition, newIndex := replacePlaceholders(w.condition, argIndex)
			sb.WriteString(condition)
			args = append(args, w.args...)
			argIndex = newIndex
		}
	}

	// RETURNING clause
	if len(d.returning) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(d.returning, ", "))
	}

	return sb.String(), args, nil
}

// MustBuild generates the SQL query and panics on error.
func (d *DeleteBuilder) MustBuild() (string, []any) {
	sql, args, err := d.Build()
	if err != nil {
		panic(fmt.Sprintf("query build error: %v", err))
	}
	return sql, args
}

// SQL returns only the SQL string (for debugging).
// Returns empty string if build fails.
func (d *DeleteBuilder) SQL() string {
	sql, _, err := d.Build()
	if err != nil {
		return ""
	}
	return sql
}

// Args returns only the arguments (for debugging).
// Returns nil if build fails.
func (d *DeleteBuilder) Args() []any {
	_, args, err := d.Build()
	if err != nil {
		return nil
	}
	return args
}
