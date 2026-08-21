package cloudip

import "testing"

func TestIsDatacenter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ip   string
		want bool
	}{
		{"", false},
		{"not-an-ip", false},
		{"127.0.0.1", false},
		{"192.168.1.1", false},
		{"1.2.3.4", false},
		{"36.149.1.1", false},
		{"114.114.114.114", false},
		{"54.179.125.189", true},
		{"124.221.108.229", true},
		{"103.151.172.76", true},
		{"5.34.216.211", true},
		{"152.175.28.50", true},
		{"108.181.23.235", true},
		{"3.0.205.115", true},
		{"47.96.1.1", true},
		{"32.216.1.1", false},
		{"8.9.1.1", false},
		{"67.159.1.1", false},
		{"2409:8c00::1", false},
		{"134.195.200.1", false},
		{"154.9.1.1", false},
		{"175.24.1.1", true},
		{"175.27.1.1", true},
		{"175.25.1.1", false},
		{"175.26.1.1", false},
		{"49.4.1.1", true},
		{"49.4.130.1", false},
		{"2600:1f13:800:2::1", true},
		{"2a01:4f8:160::1", true},
	}
	for _, tc := range cases {
		if got := IsDatacenter(tc.ip); got != tc.want {
			t.Fatalf("IsDatacenter(%q)=%v, want %v", tc.ip, got, tc.want)
		}
	}
}
