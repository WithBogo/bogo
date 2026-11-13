package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	checkKeywordRegex = regexp.MustCompile(`(?i)\bCHECK\s*\(`)
	checkAndWordRegex = regexp.MustCompile(`(?i)^\s+AND\s+`)
	checkCompareRegex = regexp.MustCompile(`^(\w+)\s*(>=|<=|>|<)\s*(-?\d+(?:\.\d+)?)$`)
	checkInRegex      = regexp.MustCompile(`(?i)^(\w+)\s+IN\s*\((.*)\)$`)
	numberTokenRegex  = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
)

// checkRules is what a CHECK expression says about one column
type checkRules struct {
	gte, gt, lte, lt *float64
	oneOf            []string
}

// parseCheckConstraint mirrors the inline CHECK constraints of a column line into validation rules.
//
// A CHECK is only mirrored when the whole expression is understood: a conjunction (AND) of
// comparisons against a number and IN lists, all on this column. A partly understood
// expression would enforce rules the database does not (for example "a >= 1 OR b = 0" is not
// "a >= 1"), so anything else is skipped and left to the database.
func parseCheckConstraint(column *Column, line string) {
	for _, body := range checkBodies(line) {
		rules, ok := parseCheckBody(body, column.Name)
		if !ok {
			continue
		}
		mergeCheckRules(column, rules)
	}
}

// checkBodies returns the text inside the parentheses of every CHECK (...) on the line,
// matching parentheses properly and ignoring those inside quoted strings
func checkBodies(line string) []string {
	var bodies []string
	for _, loc := range checkKeywordRegex.FindAllStringIndex(line, -1) {
		if body, ok := balancedParens(line, loc[1]-1); ok {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

// balancedParens returns the text between the parenthesis at open and its matching one
func balancedParens(s string, open int) (string, bool) {
	depth := 0
	inQuote := false
	for i := open; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'':
			inQuote = !inQuote // an escaped '' toggles twice, which keeps the state right
		case inQuote:
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return s[open+1 : i], true
			}
		}
	}
	return "", false
}

// stripOuterParens removes parentheses that wrap the whole expression
func stripOuterParens(expr string) string {
	for {
		expr = strings.TrimSpace(expr)
		if !strings.HasPrefix(expr, "(") {
			return expr
		}
		inner, ok := balancedParens(expr, 0)
		if !ok || len(inner)+2 != len(expr) {
			return expr
		}
		expr = inner
	}
}

// splitTopLevelAnd splits an expression at every AND that is outside parentheses and quotes
func splitTopLevelAnd(expr string) []string {
	var parts []string
	depth := 0
	inQuote := false
	start := 0

	for i := 0; i < len(expr); i++ {
		switch c := expr[i]; {
		case c == '\'':
			inQuote = !inQuote
		case inQuote:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && (c == ' ' || c == '\t'):
			if loc := checkAndWordRegex.FindStringIndex(expr[i:]); loc != nil {
				parts = append(parts, expr[start:i])
				i += loc[1] - 1
				start = i + 1
			}
		}
	}
	return append(parts, expr[start:])
}

// parseCheckBody parses one CHECK expression for a column; ok is false when any part is unsupported
func parseCheckBody(body, columnName string) (checkRules, bool) {
	var rules checkRules

	for _, clause := range splitTopLevelAnd(stripOuterParens(body)) {
		clause = stripOuterParens(clause)

		if m := checkCompareRegex.FindStringSubmatch(clause); m != nil {
			if !strings.EqualFold(m[1], columnName) {
				return checkRules{}, false
			}
			value, err := strconv.ParseFloat(m[3], 64)
			if err != nil {
				return checkRules{}, false
			}
			switch m[2] {
			case ">=":
				rules.gte = tighter(rules.gte, value, math.Max)
			case ">":
				rules.gt = tighter(rules.gt, value, math.Max)
			case "<=":
				rules.lte = tighter(rules.lte, value, math.Min)
			case "<":
				rules.lt = tighter(rules.lt, value, math.Min)
			}
			continue
		}

		if m := checkInRegex.FindStringSubmatch(clause); m != nil {
			if !strings.EqualFold(m[1], columnName) {
				return checkRules{}, false
			}
			values, ok := parseInList(m[2])
			if !ok || !oneOfSafe(values) {
				return checkRules{}, false
			}
			rules.oneOf = intersect(rules.oneOf, values)
			continue
		}

		return checkRules{}, false
	}
	return rules, true
}

// tighter keeps the stricter of two limits: pick is math.Max for lower and math.Min for upper bounds
func tighter(current *float64, candidate float64, pick func(a, b float64) float64) *float64 {
	if current != nil {
		candidate = pick(*current, candidate)
	}
	return &candidate
}

// intersect narrows an IN list by a second one; the first list is taken as is
func intersect(current, next []string) []string {
	if current == nil {
		return next
	}
	var both []string
	for _, v := range current {
		for _, w := range next {
			if v == w {
				both = append(both, v)
				break
			}
		}
	}
	return both
}

// mergeCheckRules applies parsed rules to the column, keeping the strictest limits when
// the column has several CHECK constraints
func mergeCheckRules(column *Column, rules checkRules) {
	if rules.gte != nil {
		column.Gte = tighter(column.Gte, *rules.gte, math.Max)
	}
	if rules.gt != nil {
		column.Gt = tighter(column.Gt, *rules.gt, math.Max)
	}
	if rules.lte != nil {
		column.Lte = tighter(column.Lte, *rules.lte, math.Min)
	}
	if rules.lt != nil {
		column.Lt = tighter(column.Lt, *rules.lt, math.Min)
	}
	if rules.oneOf != nil {
		column.OneOf = intersect(column.OneOf, rules.oneOf)
	}
}

// parseInList reads the inside of IN ('a', 'b, c', 3). Commas inside quotes belong to the
// value, a doubled single quote is an escaped quote, and unquoted items must be numbers.
func parseInList(list string) ([]string, bool) {
	var values []string
	var item strings.Builder
	inQuote, quoted := false, false

	flush := func() bool {
		text := strings.TrimSpace(item.String())
		switch {
		case quoted:
			values = append(values, item.String())
		case numberTokenRegex.MatchString(text):
			values = append(values, text)
		default:
			return false
		}
		item.Reset()
		quoted = false
		return true
	}

	for i := 0; i < len(list); i++ {
		c := list[i]
		switch {
		case inQuote && c == '\'' && i+1 < len(list) && list[i+1] == '\'':
			item.WriteByte('\'')
			i++
		case c == '\'':
			if !inQuote && strings.TrimSpace(item.String()) != "" {
				return nil, false
			}
			if !inQuote {
				item.Reset()
				quoted = true
			}
			inQuote = !inQuote
		case inQuote:
			item.WriteByte(c)
		case c == ',':
			if !flush() {
				return nil, false
			}
		case quoted && c != ' ' && c != '\t':
			return nil, false // text after the closing quote
		case !quoted:
			item.WriteByte(c)
		}
	}
	if inQuote || !flush() {
		return nil, false
	}
	return values, true
}

// oneOfSafe reports whether every value can be written into a validator tag inside a Go
// struct tag. Quotes, backslashes, backticks and control characters cannot, and a partial
// list would reject values the database accepts, so one unsafe value disables the whole rule.
func oneOfSafe(values []string) bool {
	for _, v := range values {
		if v == "" {
			return false
		}
		for _, r := range v {
			if r == '`' || r == '"' || r == '\'' || r == '\\' || unicode.IsControl(r) {
				return false
			}
		}
	}
	return true
}

// escapeTagParam writes a value into a validator parameter. The validator splits on commas,
// pipes and spaces, so commas and pipes use its hex escapes and values with spaces are quoted.
func escapeTagParam(v string) string {
	v = strings.ReplaceAll(v, ",", "0x2C")
	v = strings.ReplaceAll(v, "|", "0x7C")
	if strings.ContainsAny(v, " \t") {
		v = "'" + v + "'"
	}
	return v
}

func allIntegers(values []string) bool {
	for _, v := range values {
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return false
		}
	}
	return true
}

func formatBound(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func isNumericType(goType string) bool {
	return goType == "int64" || goType == "float64"
}

// effectiveBounds returns the numeric limits as the validator will check them. Integer
// columns only accept integral limits, so a fractional limit becomes the equivalent integral
// one: "age >= 1.5" is "age >= 2" and "age < 2.5" is "age < 3".
func (c Column) effectiveBounds() (gte, gt, lte, lt *float64) {
	gte, gt, lte, lt = c.Gte, c.Gt, c.Lte, c.Lt
	if c.GoType != "int64" {
		return
	}
	round := func(p *float64, f func(float64) float64) *float64 {
		if p == nil {
			return nil
		}
		v := f(*p)
		return &v
	}
	return round(gte, math.Ceil), round(gt, math.Floor), round(lte, math.Floor), round(lt, math.Ceil)
}

// appliedOneOf returns the IN list when it becomes a oneof rule on this column
func (c Column) appliedOneOf() []string {
	switch c.GoType {
	case "string":
		return c.OneOf
	case "int64":
		if allIntegers(c.OneOf) {
			return c.OneOf
		}
	}
	return nil
}

// hasNumericRules reports whether the column carries range or IN rules on a number
func (c Column) hasNumericRules() bool {
	if !isNumericType(c.GoType) {
		return false
	}
	gte, gt, lte, lt := c.effectiveBounds()
	return gte != nil || gt != nil || lte != nil || lt != nil || len(c.appliedOneOf()) > 0
}

// zeroSatisfiesRules reports whether 0 is a valid value under the numeric rules of the column
func (c Column) zeroSatisfiesRules() bool {
	gte, gt, lte, lt := c.effectiveBounds()
	if gte != nil && *gte > 0 || gt != nil && *gt >= 0 || lte != nil && *lte < 0 || lt != nil && *lt <= 0 {
		return false
	}
	if oneOf := c.appliedOneOf(); len(oneOf) > 0 && c.GoType == "int64" {
		for _, v := range oneOf {
			if n, _ := strconv.ParseInt(v, 10, 64); n == 0 {
				return true
			}
		}
		return false
	}
	return true
}

// requiresValue reports whether the database insists on a value for this column
func (c Column) requiresValue() bool {
	return !c.IsNullable && !c.IsPrimaryKey && c.DefaultValue == ""
}

// usesPointer reports whether the DTO field must be a pointer. An optional number whose rules
// reject 0 has to tell "not sent" from "sent as 0": with omitempty a plain int64 would skip
// validation for an explicit 0 and let it through to the database.
func (c Column) usesPointer() bool {
	return c.hasNumericRules() && !c.zeroSatisfiesRules() && !c.requiresValue()
}

// buildValidateTag derives go-playground/validator rules from the column definition.
//
// "required" is only used where the zero value is never legitimate (strings and timestamps).
// A mandatory number whose rules reject 0 is validated even when it is 0, which also covers
// a missing field. Optional fields are validated only when a value was sent.
func buildValidateTag(column Column) string {
	var rules []string

	needsPresence := false
	if column.requiresValue() && (column.GoType == "string" || column.GoType == "time.Time") {
		rules = append(rules, "required")
		needsPresence = true
	}
	if column.requiresValue() && column.hasNumericRules() && !column.zeroSatisfiesRules() {
		needsPresence = true // no omitempty: 0 must be checked as well
	}

	switch column.GoType {
	case "string":
		if column.MaxLength > 0 {
			rules = append(rules, fmt.Sprintf("max=%d", column.MaxLength))
		}
		if len(column.OneOf) > 0 {
			rules = append(rules, "oneof="+joinOneOf(column.OneOf))
		}
		if strings.HasPrefix(strings.ToUpper(column.Type), "UUID") {
			rules = append(rules, "uuid")
		}
	case "int64", "float64":
		gte, gt, lte, lt := column.effectiveBounds()
		if gte != nil {
			rules = append(rules, "gte="+formatBound(*gte))
		}
		if gt != nil {
			rules = append(rules, "gt="+formatBound(*gt))
		}
		if lte != nil {
			rules = append(rules, "lte="+formatBound(*lte))
		}
		if lt != nil {
			rules = append(rules, "lt="+formatBound(*lt))
		}
		if oneOf := column.appliedOneOf(); column.GoType == "int64" && len(oneOf) > 0 {
			rules = append(rules, "oneof="+strings.Join(oneOf, " "))
		}
	}

	if len(rules) == 0 {
		return ""
	}
	if !needsPresence {
		// Optional field: only validate when a value was actually sent
		rules = append([]string{"omitempty"}, rules...)
	}
	return strings.Join(rules, ",")
}

// joinOneOf formats the values of a oneof rule
func joinOneOf(values []string) string {
	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = escapeTagParam(v)
	}
	return strings.Join(escaped, " ")
}
