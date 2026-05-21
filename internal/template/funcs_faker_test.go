//go:build unit

package template

import (
	"strings"
	"testing"
)

func TestNewFakerCategories_Date(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	for _, variant := range []string{"past", "future", "recent", "birthdate"} {
		result, err := e.RenderBodyTemplate(`{{faker "date.`+variant+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker date.%s: %v", variant, err)
		}
		if len(result) != 10 { // YYYY-MM-DD
			t.Errorf("faker date.%s: expected 10-char date, got %q", variant, result)
		}
	}
}

func TestNewFakerCategories_DateDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "date"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 10 {
		t.Errorf("expected 10-char date, got %q", result)
	}
}

func TestNewFakerCategories_Finance(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	fields := map[string]func(string) bool{
		"amount":         func(s string) bool { return strings.Contains(s, ".") },
		"currency":       func(s string) bool { return len(s) == 3 },
		"currencySymbol": func(s string) bool { return len(s) >= 1 },
		"iban":           func(s string) bool { return strings.HasPrefix(s, "GB") },
		"creditCard":     func(s string) bool { return strings.HasPrefix(s, "4") },
	}

	for field, check := range fields {
		result, err := e.RenderBodyTemplate(`{{faker "finance.`+field+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker finance.%s: %v", field, err)
		}
		if !check(result) {
			t.Errorf("faker finance.%s: unexpected result %q", field, result)
		}
	}
}

func TestNewFakerCategories_FinanceDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "finance"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, ".") {
		t.Errorf("expected amount with decimal, got %q", result)
	}
}

func TestNewFakerCategories_Product(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	fields := map[string]func(string) bool{
		"name":     func(s string) bool { return len(s) > 5 },
		"category": func(s string) bool { return len(s) > 2 },
		"price":    func(s string) bool { return strings.Contains(s, ".") },
		"sku":      func(s string) bool { return strings.HasPrefix(s, "SKU-") },
	}

	for field, check := range fields {
		result, err := e.RenderBodyTemplate(`{{faker "product.`+field+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker product.%s: %v", field, err)
		}
		if !check(result) {
			t.Errorf("faker product.%s: unexpected result %q", field, result)
		}
	}
}

func TestNewFakerCategories_ProductDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "product"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) < 5 {
		t.Errorf("expected a product name, got %q", result)
	}
}

func TestNewFakerCategories_Location(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	fields := map[string]func(string) bool{
		"country":     func(s string) bool { return len(s) > 2 },
		"countryCode": func(s string) bool { return len(s) == 2 },
		"timezone":    func(s string) bool { return strings.Contains(s, "/") },
		"latitude":    func(s string) bool { return strings.Contains(s, ".") },
		"longitude":   func(s string) bool { return strings.Contains(s, ".") },
	}

	for field, check := range fields {
		result, err := e.RenderBodyTemplate(`{{faker "location.`+field+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker location.%s: %v", field, err)
		}
		if !check(result) {
			t.Errorf("faker location.%s: unexpected result %q", field, result)
		}
	}
}

func TestNewFakerCategories_LocationDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "location"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) < 2 {
		t.Errorf("expected a country name, got %q", result)
	}
}

func TestNewFakerCategories_ID(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	cases := map[string]int{
		"objectId": 24,
		"nanoid":   21,
		"shortId":  8,
	}

	for field, expectedLen := range cases {
		result, err := e.RenderBodyTemplate(`{{faker "id.`+field+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker id.%s: %v", field, err)
		}
		if len(result) != expectedLen {
			t.Errorf("faker id.%s: expected len %d, got %d: %q", field, expectedLen, len(result), result)
		}
	}
}

func TestNewFakerCategories_IDDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "id"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 36 { // UUID
		t.Errorf("expected UUID (36 chars), got %q", result)
	}
}

func TestNewFakerCategories_Color(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	hexResult, err := e.RenderBodyTemplate(`{{faker "color.hex"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hexResult, "#") || len(hexResult) != 7 {
		t.Errorf("faker color.hex: expected #rrggbb, got %q", hexResult)
	}

	nameResult, err := e.RenderBodyTemplate(`{{faker "color.name"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nameResult) < 3 {
		t.Errorf("faker color.name: expected name, got %q", nameResult)
	}

	rgbResult, err := e.RenderBodyTemplate(`{{faker "color.rgb"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rgbResult, "rgb(") {
		t.Errorf("faker color.rgb: expected rgb(...), got %q", rgbResult)
	}
}

func TestNewFakerCategories_ColorDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "color"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "#") {
		t.Errorf("expected hex color, got %q", result)
	}
}

func TestNewFakerCategories_Number(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	intResult, err := e.RenderBodyTemplate(`{{faker "number.int"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(intResult) == 0 {
		t.Errorf("faker number.int: expected non-empty")
	}

	floatResult, err := e.RenderBodyTemplate(`{{faker "number.float"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(floatResult, ".") {
		t.Errorf("faker number.float: expected decimal, got %q", floatResult)
	}
}

func TestNewFakerCategories_NumberDefault(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}
	result, err := e.RenderBodyTemplate(`{{faker "number"}}`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Errorf("expected non-empty")
	}
}

func TestNewFakerCategories_Internet(t *testing.T) {
	e := NewEngine()
	ctx := &Context{}

	fields := map[string]func(string) bool{
		"ip":  func(s string) bool { return strings.Count(s, ".") == 3 },
		"mac": func(s string) bool { return strings.Count(s, ":") == 5 },
	}

	for field, check := range fields {
		result, err := e.RenderBodyTemplate(`{{faker "internet.`+field+`"}}`, ctx)
		if err != nil {
			t.Fatalf("faker internet.%s: %v", field, err)
		}
		if !check(result) {
			t.Errorf("faker internet.%s: unexpected result %q", field, result)
		}
	}
}
