package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// jsonString returns s as a JSON string literal
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// sampleStringValue picks a string that satisfies every rule of the column at once
func sampleStringValue(col Column) string {
	switch {
	case len(col.OneOf) > 0:
		// the first allowed value that also fits the column length
		for _, v := range col.OneOf {
			if col.MaxLength == 0 || utf8.RuneCountInString(v) <= col.MaxLength {
				return jsonString(v)
			}
		}
		return jsonString(col.OneOf[0])
	case strings.HasPrefix(strings.ToUpper(col.Type), "UUID"):
		return `"123e4567-e89b-12d3-a456-426614174000"`
	}
	return `"x"`
}

// sampleJSONValue returns a JSON literal that satisfies the validation rules of the column
func sampleJSONValue(col Column) string {
	switch col.GoType {
	case "string":
		return sampleStringValue(col)
	case "int64", "float64":
		return formatBound(sampleNumber(col))
	case "bool":
		return "true"
	case "time.Time":
		return `"2024-01-01T00:00:00Z"`
	case "[]string":
		return `["a"]`
	}
	return `"x"`
}

// within reports whether v satisfies the limits
func within(v float64, gte, gt, lte, lt *float64) bool {
	return (gte == nil || v >= *gte) && (gt == nil || v > *gt) &&
		(lte == nil || v <= *lte) && (lt == nil || v < *lt)
}

// sampleNumber picks a number inside the CHECK limits of the column, preferring 1.
//
// Integer limits are already integral (see effectiveBounds), so the nearest valid integer is
// simply the limit. For floats a strict limit has no nearest value, so a value that has room
// on both sides is chosen: the midpoint between two limits, or one unit away from a single one.
func sampleNumber(col Column) float64 {
	gte, gt, lte, lt := col.effectiveBounds()

	if oneOf := col.appliedOneOf(); col.GoType == "int64" && len(oneOf) > 0 {
		first, _ := strconv.ParseFloat(oneOf[0], 64)
		for _, v := range oneOf {
			if n, _ := strconv.ParseFloat(v, 64); within(n, gte, gt, lte, lt) {
				return n
			}
		}
		return first
	}

	if within(1, gte, gt, lte, lt) {
		return 1
	}

	if col.GoType == "int64" {
		// inclusive integer range, then 1 moved into it
		lo, hi := math.Inf(-1), math.Inf(1)
		if gte != nil {
			lo = *gte
		}
		if gt != nil {
			lo = math.Max(lo, *gt+1)
		}
		if lte != nil {
			hi = *lte
		}
		if lt != nil {
			hi = math.Min(hi, *lt-1)
		}
		return math.Min(math.Max(1, lo), hi)
	}

	// floats: the tightest limit on each side. A strict limit has no nearest value, so pick one
	// with room on both sides: the midpoint between two limits, or one unit away from a single one.
	var lo, hi float64
	hasLo, hasHi := false, false
	for _, limit := range []*float64{gte, gt} {
		if limit != nil && (!hasLo || *limit > lo) {
			lo, hasLo = *limit, true
		}
	}
	for _, limit := range []*float64{lte, lt} {
		if limit != nil && (!hasHi || *limit < hi) {
			hi, hasHi = *limit, true
		}
	}

	switch {
	case hasLo && hasHi:
		return (lo + hi) / 2
	case hasLo:
		return lo + 1
	case hasHi:
		return hi - 1
	}
	return 1
}

// validRequestBody builds a JSON body that passes every validation rule
func validRequestBody(table Table) string {
	return validRequestBodyWith(table, nil)
}

// validRequestBodyWith is validRequestBody with some fields replaced by raw JSON values
func validRequestBodyWith(table Table, overrides map[string]string) string {
	var fields []string
	for _, col := range dtoColumns(table) {
		key := strings.ToLower(col.Name)
		value := sampleJSONValue(col)
		if override, ok := overrides[key]; ok {
			value = override
		}
		fields = append(fields, fmt.Sprintf("%s:%s", jsonString(key), value))
	}
	return "{" + strings.Join(fields, ",") + "}"
}

// invalidBodyTest returns a handler subtest that expects a 400 for the given Go expression
func invalidBodyTest(name, structName, singularName, bodyExpr string) string {
	return fmt.Sprintf(`
	t.Run(%[4]q, func(t *testing.T) {
		svc := &mock%[1]sService{}
		h := New%[1]sHandler(context.Background(), svc)

		rec := do%[1]sRequest(h.Create%[2]s, http.MethodPost, %[3]s, nil)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %%d", rec.Code)
		}
		if svc.createCalls != 0 {
			t.Error("service must not be called when validation fails")
		}
	})
`, structName, singularName, bodyExpr, name)
}

// validationFailureTest returns handler subtests that send invalid bodies, or "" when the
// table has no rule that a request could violate
func validationFailureTest(table Table, structName, singularName string) string {
	var tests strings.Builder

	var bodyExpr string
	for _, col := range dtoColumns(table) {
		if strings.HasPrefix(col.Validate, "required") {
			bodyExpr = "`{}`" // a missing required field
			break
		}
	}
	if bodyExpr == "" {
		for _, col := range dtoColumns(table) {
			if col.GoType == "string" && col.MaxLength > 0 {
				// one character longer than the column allows
				bodyExpr = fmt.Sprintf("`{\"%s\":\"` + strings.Repeat(\"x\", %d) + `\"}`",
					strings.ToLower(col.Name), col.MaxLength+1)
				break
			}
		}
	}
	if bodyExpr != "" {
		tests.WriteString(invalidBodyTest("rejects a body that violates validation rules", structName, singularName, bodyExpr))
	}

	// an optional number whose rules reject 0 must still be validated when 0 is sent explicitly
	for _, col := range dtoColumns(table) {
		if col.usesPointer() {
			body := validRequestBodyWith(table, map[string]string{strings.ToLower(col.Name): "0"})
			name := fmt.Sprintf("rejects an explicit zero for %s", strings.ToLower(col.Name))
			tests.WriteString(invalidBodyTest(name, structName, singularName, strconv.Quote(body)))
		}
	}

	return tests.String()
}
