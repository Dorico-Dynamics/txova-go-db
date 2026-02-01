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

// validateInsert checks that the insert has valid table, columns, and values.
func (i *InsertBuilder) validateInsert() error {
	if err := validateTableName(i.table); err != nil {
		return err
	}
	if len(i.columns) == 0 {
		return fmt.Errorf("no columns specified for insert")
	}
	for _, col := range i.columns {
		if err := i.validateColumnName(col); err != nil {
			return err
		}
	}
	if len(i.values) == 0 {
		return fmt.Errorf("no values specified for insert")
	}
	for idx, row := range i.values {
		if len(row) != len(i.columns) {
			return fmt.Errorf("row %d has %d values but %d columns specified", idx, len(row), len(i.columns))
		}
	}
	return nil
}

// buildValuesClause generates the VALUES portion and collects arguments.
func (i *InsertBuilder) buildValuesClause() (string, []any) {
	var args []any
	argIndex := 1
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

	return strings.Join(valueParts, ", "), args
}

// buildOnConflictClause generates the ON CONFLICT portion.
func (i *InsertBuilder) buildOnConflictClause() string {
	if i.onConflict == nil {
		return ""
	}
	var sb strings.Builder
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
	return sb.String()
}

// Build generates the SQL query and returns it with the arguments.
func (i *InsertBuilder) Build() (string, []any, error) {
	if err := i.validateInsert(); err != nil {
		return "", nil, err
	}

	valuesClause, args := i.buildValuesClause()

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(i.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(i.columns, ", "))
	sb.WriteString(") VALUES ")
	sb.WriteString(valuesClause)
	sb.WriteString(i.buildOnConflictClause())

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
// Returns empty string if build fails.
func (i *InsertBuilder) SQL() string {
	sql, _, err := i.Build()
	if err != nil {
		return ""
	}
	return sql
}

// Args returns only the arguments (for debugging).
// Returns nil if build fails.
func (i *InsertBuilder) Args() []any {
	_, args, err := i.Build()
	if err != nil {
		return nil
	}
	return args
}
