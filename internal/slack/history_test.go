package slack

import "testing"

func TestIsChannelID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"C0123ABCD", true},
		{"G0123ABCD", true},
		{"D0123ABCD", true},
		{"general", false},
		{"#general", false},
		{"c0123abcd", false}, // lowercase is not an ID
		{"C", false},         // prefix alone is not enough
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := IsChannelID(tt.input); got != tt.want {
				t.Errorf("IsChannelID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
