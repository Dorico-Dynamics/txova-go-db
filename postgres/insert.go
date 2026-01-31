// Package postgres provides PostgreSQL database utilities.
package postgres

import (
	"fmt"
	"strings"
)

// InsertBuilder builds INSERT queries.
type InsertBuilder struct {
	*QueryBuilder
	table      string
	columns    []string
	values     [][]any
	returning  []string
	onConflict *conflictClause
}

// conflictClause represents ON CONFLICT handling.
type conflictClause struct {
	columns    []string
	doNothing  bool
	doUpdate   []setClause
	constraint string
}

// setClause represents a column = value pair.
type setClause struct {
	column string
	value  any
}

// Insert creates a new InsertBuilder for the specified table.
func Insert(table string) *InsertBuilder {
	return &InsertBuilder{
		QueryBuilder: NewQueryBuilder(),
		table:        table,
		columns:      []string{},
		values:       [][]any{},
		returning:    []string{},
	}
}

// InsertWithAllowlist creates a new InsertBuilder with column validation.
func InsertWithAllowlist(table string, allowedColumns ...string) *InsertBuilder {
	return &InsertBuilder{
		QueryBuilder: NewQueryBuilder(allowedColumns...),
		table:        table,
		columns:      []string{},
		values:       [][]any{},
		returning:    []string{},
	}
}

// Columns specifies the columns to insert into.
func (i *InsertBuilder) Columns(columns ...string) *InsertBuilder {
	i.columns = columns
	return i
}

// Values adds a row of values to insert.
func (i *InsertBuilder) Values(values ...any) *InsertBuilder {
	i.values = append(i.values, values)
	return i
}

// Returning specifies columns to return after insert.
func (i *InsertBuilder) Returning(columns ...string) *InsertBuilder {
	i.returning = append(i.returning, columns...)
	return i
}

// OnConflictDoNothing adds ON CONFLICT DO NOTHING clause.
func (i *InsertBuilder) OnConflictDoNothing(columns ...string) *InsertBuilder {
	i.onConflict = &conflictClause{
		columns:   columns,
		doNothing: true,
	}
	return i
}

// OnConflictConstraintDoNothing adds ON CONFLICT ON CONSTRAINT ... DO NOTHING.
func (i *InsertBuilder) OnConflictConstraintDoNothing(constraint string) *InsertBuilder {
	i.onConflict = &conflictClause{
		constraint: constraint,
		doNothing:  true,
	}
	return i
}

// Build generates the SQL query and returns it with the arguments.
func (i *InsertBuilder) Build() (string, []any, error) {
	// Validate table name
	if err := validateTableName(i.table); err != nil {
		return "", nil, err
	}

	// Validate columns
	if len(i.columns) == 0 {
		return "", nil, fmt.Errorf("no columns specified for insert")
	}

	// Validate columns if allowlist is enabled
	for _, col := range i.columns {
		if err := i.validateColumnName(col); err != nil {
			return "", nil, err
		}
	}

	// Validate values
	if len(i.values) == 0 {
		return "", nil, fmt.Errorf("no values specified for insert")
	}

	// Validate each row has correct number of values
	for idx, row := range i.values {
		if len(row) != len(i.columns) {
			return "", nil, fmt.Errorf("row %d has %d values but %d columns specified",
				idx, len(row), len(i.columns))
		}
	}

	var args []any
	argIndex := 1

	var sb strings.Builder

	// INSERT INTO clause
	sb.WriteString("INSERT INTO ")
	sb.WriteString(i.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(i.columns, ", "))
	sb.WriteString(") VALUES ")

	// VALUES clause
	valueParts := make([]string, len(i.values))
	for rowIdx, row := range i.values {
		placeholders := make([]string, len(row))
		for colIdx := range row {
			placeholders[colIdx] = fmt.Sprintf("$%d", argIndex)
			argIndex++
		}
		valueParts[rowIdx] = "(" + strings.Join(placeholders, ", ") + ")"
		args = append(args, row...)
	}
	sb.WriteString(strings.Join(valueParts, ", "))

	// ON CONFLICT clause
	if i.onConflict != nil {
		sb.WriteString(" ON CONFLICT")
		if i.onConflict.constraint != "" {
			sb.WriteString(" ON CONSTRAINT ")
			sb.WriteString(i.onConflict.constraint)
		} else if len(i.onConflict.columns) > 0 {
			sb.WriteString(" (")
			sb.WriteString(strings.Join(i.onConflict.columns, ", "))
			sb.WriteString(")")
		}
		if i.onConflict.doNothing {
			sb.WriteString(" DO NOTHING")
		}
	}

	// RETURNING clause
	if len(i.returning) > 0 {
		sb.WriteString(" RETURNING ")
		sb.WriteString(strings.Join(i.returning, ", "))
	}

	return sb.String(), args, nil
}

// MustBuild generates the SQL query and panics on error.
func (i *InsertBuilder) MustBuild() (string, []any) {
	sql, args, err := i.Build()
	if err != nil {
		panic(fmt.Sprintf("query build error: %v", err))
	}
	return sql, args
}

// SQL returns only the SQL string (for debugging).
func (i *InsertBuilder) SQL() string {
	sql, _, _ := i.Build()
	return sql
}

// Args returns only the arguments (for debugging).
func (i *InsertBuilder) Args() []any {
	_, args, _ := i.Build()
	return args
}
