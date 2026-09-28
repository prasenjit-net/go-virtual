package models

import "testing"

func TestResponseConfigEffectiveKind(t *testing.T) {
	if got := (*ResponseConfig)(nil).EffectiveKind(); got != ResponseKindManual {
		t.Fatalf("nil EffectiveKind = %q, want %q", got, ResponseKindManual)
	}
	if got := (&ResponseConfig{}).EffectiveKind(); got != ResponseKindManual {
		t.Fatalf("empty Kind EffectiveKind = %q, want %q", got, ResponseKindManual)
	}
	if got := (&ResponseConfig{Kind: "bogus"}).EffectiveKind(); got != ResponseKindManual {
		t.Fatalf("unrecognised Kind EffectiveKind = %q, want %q", got, ResponseKindManual)
	}
	if got := (&ResponseConfig{Kind: ResponseKindCollection}).EffectiveKind(); got != ResponseKindCollection {
		t.Fatalf("collection Kind EffectiveKind = %q, want %q", got, ResponseKindCollection)
	}
	if (&ResponseConfig{Kind: ResponseKindCollection}).IsCollectionResponse() != true {
		t.Fatal("IsCollectionResponse() should be true for kind=collection")
	}
	if (&ResponseConfig{}).IsCollectionResponse() != false {
		t.Fatal("IsCollectionResponse() should be false for manual")
	}
}

func TestCollectionResponseConfigValidate_Valid(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{
			CollectionName: "users",
			FilterRules: []CollectionFilter{
				{TargetPath: "tenantId", Value: ValueBinding{Source: ValueSourceHeader, Key: "X-Tenant-Id"}},
			},
		},
		AdditionalMappers: []NamedQuery{
			{
				OutputKey: "plan",
				Mode:      QueryModeFindOne,
				CollectionQuery: CollectionQuery{
					CollectionName: "plans",
					FilterRules: []CollectionFilter{
						{TargetPath: "_id", Value: ValueBinding{Source: ValueSourcePrimary, Key: "planId"}},
					},
				},
			},
		},
		Overrides: []FieldOverride{
			{TargetPath: "displayName", Value: ValueBinding{Source: ValueSourceDocument, Key: "profile.name"}},
			{TargetPath: "planLabel", Value: ValueBinding{Source: ValueSourceMapper, Key: "plan.label"}},
			{TargetPath: "requested", Value: ValueBinding{Source: ValueSourceQuery, Key: "include"}},
			{TargetPath: "active", Value: ValueBinding{Source: ValueSourceLiteral, Value: []byte("true")}},
		},
	}
	if errs := cr.Validate(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestCollectionResponseConfigValidate_MissingCollectionName(t *testing.T) {
	cr := &CollectionResponseConfig{}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error for missing primary.collectionName")
	}
}

func TestCollectionResponseConfigValidate_FilterSourceNotAllowed(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{
			CollectionName: "users",
			FilterRules: []CollectionFilter{
				{TargetPath: "id", Value: ValueBinding{Source: ValueSourcePrimary, Key: "x"}},
			},
		},
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error: primary filters cannot use source=primary")
	}
}

func TestCollectionResponseConfigValidate_OverrideDocumentSourceNotAllowedInFilter(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{
			CollectionName: "users",
			FilterRules: []CollectionFilter{
				{TargetPath: "id", Value: ValueBinding{Source: ValueSourceDocument, Key: "x"}},
			},
		},
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error: primary filters cannot use source=document")
	}
}

func TestCollectionResponseConfigValidate_InvalidLiteralJSON(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{CollectionName: "users"},
		Overrides: []FieldOverride{
			{TargetPath: "active", Value: ValueBinding{Source: ValueSourceLiteral, Value: []byte("not-json")}},
		},
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error for invalid literal JSON")
	}
}

func TestCollectionResponseConfigValidate_WholeMapperOutput(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{CollectionName: "users"},
		AdditionalMappers: []NamedQuery{
			{OutputKey: "plan", Mode: QueryModeFindOne, CollectionQuery: CollectionQuery{CollectionName: "plans"}},
		},
		Overrides: []FieldOverride{
			{TargetPath: "planLabel", Value: ValueBinding{Source: ValueSourceMapper, Key: "plan"}},
		},
	}
	errs := cr.Validate()
	if len(errs) != 0 {
		t.Fatalf("whole mapper output rejected: %v", errs)
	}
	for _, key := range []string{"", ".plan", "plan.", "plan..name", "unknown"} {
		cr.Overrides[0].Value.Key = key
		if len(cr.Validate()) == 0 {
			t.Fatalf("invalid mapper key accepted: %q", key)
		}
	}
}

func TestCollectionResponseConfigValidate_MapperReferencesUnknownOutputKey(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{CollectionName: "users"},
		Overrides: []FieldOverride{
			{TargetPath: "planLabel", Value: ValueBinding{Source: ValueSourceMapper, Key: "plan.label"}},
		},
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error: mapper output key not declared in additionalMappers")
	}
}

func TestCollectionResponseConfigValidate_DuplicateOutputKeys(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary: CollectionQuery{CollectionName: "users"},
		AdditionalMappers: []NamedQuery{
			{OutputKey: "plan", Mode: QueryModeFindOne, CollectionQuery: CollectionQuery{CollectionName: "plans"}},
			{OutputKey: "plan", Mode: QueryModeFindOne, CollectionQuery: CollectionQuery{CollectionName: "plans2"}},
		},
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error for duplicated additionalMappers outputKey")
	}
}

func TestCollectionResponseConfigValidate_BadRootKind(t *testing.T) {
	cr := &CollectionResponseConfig{
		Primary:  CollectionQuery{CollectionName: "users"},
		RootKind: "list",
	}
	errs := cr.Validate()
	if len(errs) == 0 {
		t.Fatal("expected an error for invalid rootKind")
	}
}

func TestCollectionResponseConfigValidate_NilReceiver(t *testing.T) {
	var cr *CollectionResponseConfig
	errs := cr.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error for nil config, got %v", errs)
	}
}

func TestEffectiveFallbackToExample(t *testing.T) {
	var cr *CollectionResponseConfig
	if !cr.EffectiveFallbackToExample() {
		t.Fatal("nil config should default to true")
	}
	cr = &CollectionResponseConfig{}
	if !cr.EffectiveFallbackToExample() {
		t.Fatal("unset FallbackToExample should default to true")
	}
	f := false
	cr.FallbackToExample = &f
	if cr.EffectiveFallbackToExample() {
		t.Fatal("explicit false should be honored")
	}
}

func TestCollectionResponseWriteValidation(t *testing.T) {
	rule := CollectionFilter{TargetPath: "name", Value: ValueBinding{Source: ValueSourceLiteral, Value: []byte(`"value"`)}}
	cases := []struct {
		name          string
		mode          CollectionOpType
		filters, data []CollectionFilter
		valid         bool
	}{
		{"insert", ColOpInsert, nil, []CollectionFilter{rule}, true},
		{"update", ColOpUpdate, []CollectionFilter{rule}, []CollectionFilter{rule}, true},
		{"upsert", ColOpUpsert, []CollectionFilter{rule}, []CollectionFilter{rule}, true},
		{"delete", ColOpDelete, []CollectionFilter{rule}, nil, true},
		{"insert filters", ColOpInsert, []CollectionFilter{rule}, []CollectionFilter{rule}, false},
		{"update no filter", ColOpUpdate, nil, []CollectionFilter{rule}, false},
		{"upsert no data", ColOpUpsert, []CollectionFilter{rule}, nil, false},
		{"delete data", ColOpDelete, []CollectionFilter{rule}, []CollectionFilter{rule}, false},
		{"read data", ColOpFindOne, nil, []CollectionFilter{rule}, false},
		{"duplicate target", ColOpUpdate, []CollectionFilter{rule}, []CollectionFilter{rule, rule}, false},
		{"unknown mode", "unknown", nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &CollectionResponseConfig{Primary: CollectionQuery{CollectionName: "users"}, AdditionalMappers: []NamedQuery{{OutputKey: "result", Mode: tc.mode, CollectionQuery: CollectionQuery{CollectionName: "users", FilterRules: tc.filters, DataRules: tc.data}}}}
			if errs := cfg.Validate(); (len(errs) == 0) != tc.valid {
				t.Fatalf("valid=%v errors=%v", tc.valid, errs)
			}
		})
	}
	for _, field := range []string{"_id", "profile.name"} {
		data := rule
		data.TargetPath = field
		cfg := &CollectionResponseConfig{Primary: CollectionQuery{CollectionName: "users", Mode: ColOpUpdate, FilterRules: []CollectionFilter{rule}, DataRules: []CollectionFilter{data}}}
		if len(cfg.Validate()) == 0 {
			t.Fatalf("update target %s accepted", field)
		}
	}
}
