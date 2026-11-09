package handler

import (
	"strings"
	"testing"
)

func TestOrderReprocessBlockReason(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		configured bool
		wantEmpty  bool
		wantSubstr string
	}{
		{name: "failed ok", status: "failed", configured: true, wantEmpty: true},
		{name: "sent refused", status: "sent", configured: true, wantSubstr: "only failed"},
		{name: "claimed refused", status: "claimed", configured: true, wantSubstr: "only failed"},
		{name: "failed missing creds", status: "failed", configured: false, wantSubstr: "not configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := orderReprocessBlockReason(tc.status, tc.configured)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantSubstr) {
				t.Fatalf("got %q want substr %q", got, tc.wantSubstr)
			}
		})
	}
}
