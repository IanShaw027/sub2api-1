package main

import (
	"strings"
	"reflect"
	"testing"
)

func TestParseOrderIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []int64
		wantErr bool
	}{
		{name: "empty", raw: "   ", want: nil},
		{name: "valid list", raw: "1, 2,3", want: []int64{1, 2, 3}},
		{name: "skips blank segments", raw: "1, , 2", want: []int64{1, 2}},
		{name: "rejects partial numeric prefix", raw: "123abc", wantErr: true},
		{name: "rejects malformed item in list", raw: "1, 2x", wantErr: true},
		{name: "rejects zero", raw: "0", wantErr: true},
		{name: "rejects negative", raw: "-4", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseOrderIDs(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOrderIDs(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseOrderIDs(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBuildAffectedOrdersQueryTargetsAffiliateFailureAuditInsteadOfSpecificErrorText(t *testing.T) {
	t.Parallel()

	query, args := buildAffectedOrdersQuery(nil)

	if !strings.Contains(query, "pal.action = 'AFFILIATE_REBATE_FAILED'") {
		t.Fatalf("expected query to require AFFILIATE_REBATE_FAILED action, got:\n%s", query)
	}
	if strings.Contains(strings.ToLower(query), "inconsistent types deduced for parameter") {
		t.Fatalf("query should not depend on a specific SQL error string, got:\n%s", query)
	}
	if strings.Contains(strings.ToLower(query), "pal.detail like") {
		t.Fatalf("query should not filter by pal.detail LIKE anymore, got:\n%s", query)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args without order ids, got %v", args)
	}
}

func TestBuildAffectedOrdersQueryAppendsOrderIDFilter(t *testing.T) {
	t.Parallel()

	query, args := buildAffectedOrdersQuery([]int64{11, 22})

	if !strings.Contains(query, "AND po.id IN ($1, $2)") {
		t.Fatalf("expected order id filter placeholders in query, got:\n%s", query)
	}
	if !reflect.DeepEqual(args, []any{int64(11), int64(22)}) {
		t.Fatalf("unexpected args: got %v", args)
	}
}
