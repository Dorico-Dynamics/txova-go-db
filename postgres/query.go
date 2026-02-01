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
func (s *SelectBuilder) WhereLike(column, pattern string) *SelectBuilder {
	condition := fmt.Sprintf("%s LIKE ?", column)
	s.where = append(s.where, whereClause{condition: condition, args: []any{pattern}, isOr: false})
	return s
}

// WhereILike adds a WHERE column ILIKE pattern condition (case-insensitive).
func (s *SelectBuilder) WhereILike(column, pattern string) *SelectBuilder {
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

// WhereBetween adds a WHERE column BETWEEN minVal AND maxVal condition.
func (s *SelectBuilder) WhereBetween(column string, minVal, maxVal any) *SelectBuilder {
	condition := fmt.Sprintf("%s BETWEEN ? AND ?", column)
	s.where = append(s.where, whereClause{condition: condition, args: []any{minVal, maxVal}, isOr: false})
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

// validateSelect checks that the select has valid table and columns.
func (s *SelectBuilder) validateSelect() error {
	if err := validateTableName(s.table); err != nil {
		return err
	}
	if err := s.validateSelectColumns(); err != nil {
		return err
	}
	if err := s.validateGroupByColumns(); err != nil {
		return err
	}
	if err := s.validateOrderByColumns(); err != nil {
		return err
	}
	return nil
}

// validateSelectColumns validates the SELECT columns.
func (s *SelectBuilder) validateSelectColumns() error {
	for _, col := range s.columns {
		if col == "*" {
			continue
		}
		if err := s.validateColumnName(col); err != nil {
			return err
		}
	}
	return nil
}

// validateGroupByColumns validates the GROUP BY entries.
func (s *SelectBuilder) validateGroupByColumns() error {
	for _, entry := range s.groupBy {
		entry = strings.TrimSpace(entry)
		if entry == "" || entry == "*" {
			continue
		}
		if err := s.validateColumnName(entry); err != nil {
			return fmt.Errorf("invalid group by column: %q", entry)
		}
	}
	return nil
}

// validateOrderByColumns validates the ORDER BY entries.
func (s *SelectBuilder) validateOrderByColumns() error {
	for _, entry := range s.orderBy {
		col := strings.TrimSpace(entry.column)
		if col == "" || col == "*" {
			continue
		}
		if err := s.validateColumnName(col); err != nil {
			return fmt.Errorf("invalid order by column: %q", col)
		}
	}
	return nil
}

// buildSelectClause generates the SELECT portion.
func (s *SelectBuilder) buildSelectClause() string {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	if s.distinct {
		sb.WriteString("DISTINCT ")
	}
	if len(s.columns) == 0 {
		sb.WriteString("*")
	} else {
		sb.WriteString(strings.Join(s.columns, ", "))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(s.table)
	return sb.String()
}

// buildJoinClauses generates the JOIN portions and returns args and next arg index.
func (s *SelectBuilder) buildJoinClauses(argIndex int) (string, []any, int) {
	if len(s.joins) == 0 {
		return "", nil, argIndex
	}
	var sb strings.Builder
	var args []any
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
	return sb.String(), args, argIndex
}

// buildConditionClauses generates WHERE or HAVING clauses from whereClause slice.
func buildConditionClauses(clauses []whereClause, keyword string, argIndex int) (string, []any, int) {
	if len(clauses) == 0 {
		return "", nil, argIndex
	}
	var sb strings.Builder
	var args []any
	sb.WriteString(keyword)
	for i, w := range clauses {
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
	return sb.String(), args, argIndex
}

// buildOrderByClause generates the ORDER BY portion.
func (s *SelectBuilder) buildOrderByClause() string {
	if len(s.orderBy) == 0 {
		return ""
	}
	orderParts := make([]string, len(s.orderBy))
	for i, o := range s.orderBy {
		dir := strings.ToUpper(string(o.direction))
		if dir == "" {
			dir = "ASC"
		}
		orderParts[i] = fmt.Sprintf("%s %s", o.column, dir)
	}
	return " ORDER BY " + strings.Join(orderParts, ", ")
}

// buildLimitOffsetClause generates the LIMIT and OFFSET portions.
func (s *SelectBuilder) buildLimitOffsetClause() string {
	var sb strings.Builder
	if s.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *s.limit))
	}
	if s.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *s.offset))
	}
	return sb.String()
}

// buildLockingClause generates the FOR UPDATE/FOR SHARE portion.
func (s *SelectBuilder) buildLockingClause() string {
	if s.forUpdate {
		return " FOR UPDATE"
	}
	if s.forShare {
		return " FOR SHARE"
	}
	return ""
}

// Build generates the SQL query and returns it with the arguments.
func (s *SelectBuilder) Build() (string, []any, error) {
	if err := s.validateSelect(); err != nil {
		return "", nil, err
	}

	var sb strings.Builder
	args := make([]any, 0, len(s.joins)+len(s.where)+len(s.having))
	argIndex := 1

	sb.WriteString(s.buildSelectClause())

	joinClause, joinArgs, argIndex := s.buildJoinClauses(argIndex)
	sb.WriteString(joinClause)
	args = append(args, joinArgs...)

	whereClause, whereArgs, argIndex := buildConditionClauses(s.where, " WHERE ", argIndex)
	sb.WriteString(whereClause)
	args = append(args, whereArgs...)

	if len(s.groupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(s.groupBy, ", "))
	}

	havingClause, havingArgs, _ := buildConditionClauses(s.having, " HAVING ", argIndex)
	sb.WriteString(havingClause)
	args = append(args, havingArgs...)

	sb.WriteString(s.buildOrderByClause())
	sb.WriteString(s.buildLimitOffsetClause())
	sb.WriteString(s.buildLockingClause())

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
// Returns empty string if build fails.
func (s *SelectBuilder) SQL() string {
	sql, _, err := s.Build()
	if err != nil {
		return ""
	}
	return sql
}

// Args returns only the arguments (for debugging).
// Returns nil if build fails.
func (s *SelectBuilder) Args() []any {
	_, args, err := s.Build()
	if err != nil {
		return nil
	}
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
