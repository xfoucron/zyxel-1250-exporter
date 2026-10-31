package collector

import "testing"

func TestParseSpeed(t *testing.T) {
	for in, want := range map[string]float64{
		"10G-Full":   10e9,
		"2.5G-Full":  2.5e9,
		"1000M-Full": 1e9,
		"100M-Half":  100e6,
		"10 Gbps":    10e9,
		"100 Mbps":   100e6,
		"auto":       0,
		"":           0,
		"12":         0,
	} {
		if got := parseSpeed(in); got != want {
			t.Errorf("parseSpeed(%q) = %v, want %v", in, got, want)
		}
	}
}
