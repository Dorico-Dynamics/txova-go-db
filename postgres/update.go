// Package postgres provides PostgreSQL database utilities.
package postgres

import (
	"fmt"
	"strings"
)

// UpdateBuilder builds UPDATE queries.
type UpdateBuilder struct {
	*QueryBuilder
	table     string
	sets      []setClause
	where     []whereClause
	returning []string
}

// Update creates a new UpdateBuilder for the specified table.
func Update(table string) *UpdateBuilder {
	return &UpdateBuilder{
		QueryBuilder: NewQueryBuilder(),
		table:        table,
		sets:         []setClause{},
		where:        []whereClause{},
		returning:    []string{},
	}
}

// UpdateWithAllowlist creates a new UpdateBuilder with column validation.
func UpdateWithAllowlist(table string, allowedColumns ...string) *UpdateBuilder {
	return &UpdateBuilder{
		QueryBuilder: NewQueryBuilder(allowedColumns...),
		table:        table,
		sets:         []setClause{},
		where:        []whereClause{},
		returning:    []string{},
	}
}

// Set adds a column = value pair to update.
func (u *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	u.sets = append(u.sets, setClause{column: column, value: value})
	return u
}

// SetMap adds multiple column = value pairs from a map.
func (u *UpdateBuilder) SetMap(values map[string]any) *UpdateBuilder {
	for col, val := range values {
		u.sets = append(u.sets, setClause{column: col, value: val})
	}
	return u
}

// Where adds a WHERE condition with AND logic.
func (u *UpdateBuilder) Where(condition string, args ...any) *UpdateBuilder {
	u.where = append(u.where, whereClause{condition: condition, args: args, isOr: false})
	return u
}

// OrWhere adds a WHERE condition with OR logic.
func (u *UpdateBuilder) OrWhere(condition string, args ...any) *UpdateBuilder {
	u.where = append(u.where, whereClause{condition: condition, args: args, isOr: true})
	return u
}

// WhereIn adds a WHERE column IN (...) condition.
func (u *UpdateBuilder) WhereIn(column string, values ...any) *UpdateBuilder {
	if len(values) == 0 {
		return u
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	condition := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	u.where = append(u.where, whereClause{condition: condition, args: values, isOr: false})
	return u
}

// Returning specifies columns to return after update.
func (u *UpdateBuilder) Returning(columns ...string) *UpdateBuilder {
	u.returning = append(u.returning, columns...)
	return u
}

// Build generates the SQL query and returns it with the arguments.
func (u *UpdateBuilder) Build() (string, []any, error) {
	// Validate table name
	if err := validateTableName(u.table); err != nil {
		return "", nil, err
	}

	// Validate SET clause
	if len(u.sets) == 0 {
		return "", nil, fmt.Errorf("no columns specified for update")
	}

	// Validate columns if allowlist is enabled
	for _, s := range u.sets {
		if err := u.validateColumnName(s.column); err != nil {
			return "", nil, err
		}
	}

	var args []any
	argIndex := 1

	var sb strings.Builder

	// UPDATE clause
	sb.WriteString("UPDATE ")
	sb.WriteString(u.table)
	sb.WriteString(" SET ")

	// SET clause
	setParts := make([]string, len(u.sets))
	for i, s := range u.sets {
		setParts[i] = fmt.Sprintf("%s = $%d", s.column, argIndex)
		args = append(args, s.value)
		argIndex++
	}
	sb.WriteString(strings.Join(setParts, ", "))

	// WHERE clause
	if len(u.where) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range u.where {
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
	if len(u.returning) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(u.returning, ", "))
	}

	return sb.String(), args, nil
}

// MustBuild generates the SQL query and panics on error.
func (u *UpdateBuilder) MustBuild() (string, []any) {
	sql, args, err := u.Build()
	if err != nil {
		panic(fmt.Sprintf("query build error: %v", err))
	}
	return sql, args
}

// SQL returns only the SQL string (for debugging).
// Returns empty string if build fails.
func (u *UpdateBuilder) SQL() string {
	sql, _, err := u.Build()
	if err != nil {
		return ""
	}
	return sql
}

// Args returns only the arguments (for debugging).
// Returns nil if build fails.
func (u *UpdateBuilder) Args() []any {
	_, args, err := u.Build()
	if err != nil {
		return nil
	}
	return args
}
