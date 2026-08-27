package server

import "testing"

func TestContainsOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		wanted  string
		want    bool
	}{
		{name: "exact origin", origins: []string{"https://canvas.example.com"}, wanted: "https://canvas.example.com", want: true},
		{name: "trims values", origins: []string{"  http://localhost:18082  "}, wanted: "http://localhost:18082", want: true},
		{name: "different port", origins: []string{"https://canvas.example.com"}, wanted: "https://canvas.example.com:8443", want: false},
		{name: "empty origin", origins: []string{"https://canvas.example.com"}, wanted: " ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsOrigin(tt.origins, tt.wanted); got != tt.want {
				t.Fatalf("containsOrigin(%q, %q) = %v, want %v", tt.origins, tt.wanted, got, tt.want)
			}
		})
	}
}
