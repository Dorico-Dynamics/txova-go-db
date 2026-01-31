// Package postgres provides PostgreSQL database utilities.
package postgres

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Dorico-Dynamics/txova-go-types/pagination"
)

// columnNameRegex validates column/table names to prevent SQL injection.
// Only allows alphanumeric characters, underscores, and dots (for qualified names).
var columnNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

// JoinType represents the type of SQL JOIN.
type JoinType string

const (
	InnerJoin JoinType = "INNER JOIN"
	LeftJoin  JoinType = "LEFT JOIN"
	RightJoin JoinType = "RIGHT JOIN"
)

// join represents a JOIN clause.
type join struct {
	joinType  JoinType
	table     string
	condition string
	args      []any
}

// whereClause represents a WHERE condition.
type whereClause struct {
	condition string
	args      []any
	isOr      bool
}

// orderByClause represents an ORDER BY clause.
type orderByClause struct {
	column    string
	direction pagination.SortDirection
}

// QueryBuilder provides common functionality for all query builders.
type QueryBuilder struct {
	allowedColumns map[string]struct{}
	errors         []error
}

// NewQueryBuilder creates a new QueryBuilder with optional column allowlist.
func NewQueryBuilder(allowedColumns ...string) *QueryBuilder {
	qb := &QueryBuilder{
		allowedColumns: make(map[string]struct{}),
	}
	for _, col := range allowedColumns {
		qb.allowedColumns[col] = struct{}{}
	}
	return qb
}

// validateColumnName checks if a column name is valid and allowed.
// When an allowlist is provided, the column must be in the allowlist.
// When no allowlist is provided, only basic validation is performed for table/column names.
func (qb *QueryBuilder) validateColumnName(column string) error {
	if column == "" {
		return fmt.Errorf("column name cannot be empty")
	}
	// If an allowlist is provided, enforce strict validation
	if len(qb.allowedColumns) > 0 {
		if !columnNameRegex.MatchString(column) {
			return fmt.Errorf("invalid column name: %s", column)
		}
		if _, ok := qb.allowedColumns[column]; !ok {
			return fmt.Errorf("column not in allowlist: %s", column)
		}
	}
	// Without an allowlist, allow expressions like COUNT(*), SUM(x), etc.
	// SQL injection is prevented by parameterized queries for values.
	return nil
}

// validateTableName checks if a table name is valid.
func validateTableName(table string) error {
	if table == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	if !columnNameRegex.MatchString(table) {
		return fmt.Errorf("invalid table name: %s", table)
	}
	return nil
}

// SelectBuilder builds SELECT queries.
type SelectBuilder struct {
	*QueryBuilder
	table     string
	columns   []string
	distinct  bool
	where     []whereClause
	joins     []join
	orderBy   []orderByClause
	limit     *int
	offset    *int
	groupBy   []string
	having    []whereClause
	forUpdate bool
	forShare  bool
}

// Select creates a new SelectBuilder for the specified table.
func Select(table string) *SelectBuilder {
	return &SelectBuilder{
		QueryBuilder: NewQueryBuilder(),
		table:        table,
		columns:      []string{},
		where:        []whereClause{},
		joins:        []join{},
		orderBy:      []orderByClause{},
		groupBy:      []string{},
		having:       []whereClause{},
	}
}

// SelectWithAllowlist creates a new SelectBuilder with column validation.
func SelectWithAllowlist(table string, allowedColumns ...string) *SelectBuilder {
	return &SelectBuilder{
		QueryBuilder: NewQueryBuilder(allowedColumns...),
		table:        table,
		columns:      []string{},
		where:        []whereClause{},
		joins:        []join{},
		orderBy:      []orderByClause{},
		groupBy:      []string{},
		having:       []whereClause{},
	}
}

// Columns specifies which columns to select.
func (s *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	s.columns = append(s.columns, columns...)
	return s
}

// Distinct adds DISTINCT to the query.
func (s *SelectBuilder) Distinct() *SelectBuilder {
	s.distinct = true
	return s
}

// Where adds a WHERE condition with AND logic.
// Use ? as placeholder for arguments, they will be converted to $1, $2, etc.
func (s *SelectBuilder) Where(condition string, args ...any) *SelectBuilder {
	s.where = append(s.where, whereClause{condition: condition, args: args, isOr: false})
	return s
}

// OrWhere adds a WHERE condition with OR logic.
func (s *SelectBuilder) OrWhere(condition string, args ...any) *SelectBuilder {
	s.where = append(s.where, whereClause{condition: condition, args: args, isOr: true})
	return s
}

// WhereIn adds a WHERE column IN (...) condition.
func (s *SelectBuilder) WhereIn(column string, values ...any) *SelectBuilder {
	if len(values) == 0 {
		return s
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	condition := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	s.where = append(s.where, whereClause{condition: condition, args: values, isOr: false})
	return s
}

// WhereNotIn adds a WHERE column NOT IN (...) condition.
func (s *SelectBuilder) WhereNotIn(column string, values ...any) *SelectBuilder {
	if len(values) == 0 {
		return s
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	condition := fmt.Sprintf("%s NOT IN (%s)", column, strings.Join(placeholders, ", "))
	s.where = append(s.where, whereClause{condition: condition, args: values, isOr: false})
	return s
}

// WhereLike adds a WHERE column LIKE pattern condition.
func (s *SelectBuilder) WhereLike(column string, pattern string) *SelectBuilder {
	condition := fmt.Sprintf("%s LIKE ?", column)
	s.where = append(s.where, whereClause{condition: condition, args: []any{pattern}, isOr: false})
	return s
}

// WhereILike adds a WHERE column ILIKE pattern condition (case-insensitive).
func (s *SelectBuilder) WhereILike(column string, pattern string) *SelectBuilder {
	condition := fmt.Sprintf("%s ILIKE ?", column)
	s.where = append(s.where, whereClause{condition: condition, args: []any{pattern}, isOr: false})
	return s
}

// WhereNull adds a WHERE column IS NULL condition.
func (s *SelectBuilder) WhereNull(column string) *SelectBuilder {
	condition := fmt.Sprintf("%s IS NULL", column)
	s.where = append(s.where, whereClause{condition: condition, args: nil, isOr: false})
	return s
}

// WhereNotNull adds a WHERE column IS NOT NULL condition.
func (s *SelectBuilder) WhereNotNull(column string) *SelectBuilder {
	condition := fmt.Sprintf("%s IS NOT NULL", column)
	s.where = append(s.where, whereClause{condition: condition, args: nil, isOr: false})
	return s
}

// WhereBetween adds a WHERE column BETWEEN min AND max condition.
func (s *SelectBuilder) WhereBetween(column string, min, max any) *SelectBuilder {
	condition := fmt.Sprintf("%s BETWEEN ? AND ?", column)
	s.where = append(s.where, whereClause{condition: condition, args: []any{min, max}, isOr: false})
	return s
}

// Join adds an INNER JOIN clause.
func (s *SelectBuilder) Join(table, condition string, args ...any) *SelectBuilder {
	s.joins = append(s.joins, join{joinType: InnerJoin, table: table, condition: condition, args: args})
	return s
}

// LeftJoin adds a LEFT JOIN clause.
func (s *SelectBuilder) LeftJoin(table, condition string, args ...any) *SelectBuilder {
	s.joins = append(s.joins, join{joinType: LeftJoin, table: table, condition: condition, args: args})
	return s
}

// RightJoin adds a RIGHT JOIN clause.
func (s *SelectBuilder) RightJoin(table, condition string, args ...any) *SelectBuilder {
	s.joins = append(s.joins, join{joinType: RightJoin, table: table, condition: condition, args: args})
	return s
}

// OrderBy adds an ORDER BY clause.
func (s *SelectBuilder) OrderBy(column string, direction pagination.SortDirection) *SelectBuilder {
	s.orderBy = append(s.orderBy, orderByClause{column: column, direction: direction})
	return s
}

// OrderByAsc adds an ascending ORDER BY clause.
func (s *SelectBuilder) OrderByAsc(column string) *SelectBuilder {
	return s.OrderBy(column, pagination.SortAsc)
}

// OrderByDesc adds a descending ORDER BY clause.
func (s *SelectBuilder) OrderByDesc(column string) *SelectBuilder {
	return s.OrderBy(column, pagination.SortDesc)
}

// Limit sets the LIMIT clause.
func (s *SelectBuilder) Limit(limit int) *SelectBuilder {
	s.limit = &limit
	return s
}

// Offset sets the OFFSET clause.
func (s *SelectBuilder) Offset(offset int) *SelectBuilder {
	s.offset = &offset
	return s
}

// Page applies pagination from a PageRequest.
func (s *SelectBuilder) Page(req pagination.PageRequest) *SelectBuilder {
	normalized := req.Normalize()
	s.limit = &normalized.Limit
	s.offset = &normalized.Offset
	if normalized.SortField != "" {
		s.orderBy = append(s.orderBy, orderByClause{
			column:    normalized.SortField,
			direction: normalized.SortDir,
		})
	}
	return s
}

// GroupBy adds GROUP BY columns.
func (s *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	s.groupBy = append(s.groupBy, columns...)
	return s
}

// Having adds a HAVING condition.
func (s *SelectBuilder) Having(condition string, args ...any) *SelectBuilder {
	s.having = append(s.having, whereClause{condition: condition, args: args, isOr: false})
	return s
}

// ForUpdate adds FOR UPDATE locking.
func (s *SelectBuilder) ForUpdate() *SelectBuilder {
	s.forUpdate = true
	s.forShare = false
	return s
}

// ForShare adds FOR SHARE locking.
func (s *SelectBuilder) ForShare() *SelectBuilder {
	s.forShare = true
	s.forUpdate = false
	return s
}

// Build generates the SQL query and returns it with the arguments.
func (s *SelectBuilder) Build() (string, []any, error) {
	// Validate table name
	if err := validateTableName(s.table); err != nil {
		return "", nil, err
	}

	// Validate columns if allowlist is enabled
	for _, col := range s.columns {
		if col != "*" {
			if err := s.validateColumnName(col); err != nil {
				return "", nil, err
			}
		}
	}

	var args []any
	argIndex := 1

	var sb strings.Builder

	// SELECT clause
	sb.WriteString("SELECT ")
	if s.distinct {
		sb.WriteString("DISTINCT ")
	}
	if len(s.columns) == 0 {
		sb.WriteString("*")
	} else {
		sb.WriteString(strings.Join(s.columns, ", "))
	}

	// FROM clause
	sb.WriteString(" FROM ")
	sb.WriteString(s.table)

	// JOIN clauses
	for _, j := range s.joins {
		sb.WriteString(" ")
		sb.WriteString(string(j.joinType))
		sb.WriteString(" ")
		sb.WriteString(j.table)
		sb.WriteString(" ON ")
		joinCondition, newIndex := replacePlaceholders(j.condition, argIndex)
		sb.WriteString(joinCondition)
		args = append(args, j.args...)
		argIndex = newIndex
	}

	// WHERE clause
	if len(s.where) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range s.where {
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

	// GROUP BY clause
	if len(s.groupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(s.groupBy, ", "))
	}

	// HAVING clause
	if len(s.having) > 0 {
		sb.WriteString(" HAVING ")
		for i, h := range s.having {
			if i > 0 {
				if h.isOr {
					sb.WriteString(" OR ")
				} else {
					sb.WriteString(" AND ")
				}
			}
			condition, newIndex := replacePlaceholders(h.condition, argIndex)
			sb.WriteString(condition)
			args = append(args, h.args...)
			argIndex = newIndex
		}
	}

	// ORDER BY clause
	if len(s.orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		orderParts := make([]string, len(s.orderBy))
		for i, o := range s.orderBy {
			dir := strings.ToUpper(string(o.direction))
			if dir == "" {
				dir = "ASC"
			}
			orderParts[i] = fmt.Sprintf("%s %s", o.column, dir)
		}
		sb.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT clause
	if s.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *s.limit))
	}

	// OFFSET clause
	if s.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *s.offset))
	}

	// Locking clause
	if s.forUpdate {
		sb.WriteString(" FOR UPDATE")
	} else if s.forShare {
		sb.WriteString(" FOR SHARE")
	}

	return sb.String(), args, nil
}

// MustBuild generates the SQL query and panics on error.
func (s *SelectBuilder) MustBuild() (string, []any) {
	sql, args, err := s.Build()
	if err != nil {
		panic(fmt.Sprintf("query build error: %v", err))
	}
	return sql, args
}

// SQL returns only the SQL string (for debugging).
func (s *SelectBuilder) SQL() string {
	sql, _, _ := s.Build()
	return sql
}

// Args returns only the arguments (for debugging).
func (s *SelectBuilder) Args() []any {
	_, args, _ := s.Build()
	return args
}

// replacePlaceholders replaces ? placeholders with $1, $2, etc.
func replacePlaceholders(condition string, startIndex int) (string, int) {
	var result strings.Builder
	index := startIndex
	for _, ch := range condition {
		if ch == '?' {
			result.WriteString(fmt.Sprintf("$%d", index))
			index++
		} else {
			result.WriteRune(ch)
		}
	}
	return result.String(), index
}
