package main

import "testing"

func TestStripRoutingKeywords_KachestvaEnergii(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"качество энергии Вехюиах", "Вехюиах"},
		{"качество энергии растворяет все кармические долги", "растворяет все кармические долги"},
		{"качества энергии Вехюиах", "Вехюиах"},
		{"качество энергии, растворяет", "растворяет"}, // пунктуация после "энергии"
	}
	for _, tt := range tests {
		got := stripRoutingKeywords(tt.query, "kachestva_energii")
		if got != tt.want {
			t.Errorf("stripRoutingKeywords(%q, kachestva_energii) = %q, want %q", tt.query, got, tt.want)
		}
	}
}
