package cmd

import "testing"

func TestParseTmuxSessionList(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "parses simple list",
			input:  "dev\nops\nreview\n",
			expect: []string{"dev", "ops", "review"},
		},
		{
			name:   "trims and deduplicates",
			input:  " dev \nops\n\nops\n",
			expect: []string{"dev", "ops"},
		},
		{
			name:   "empty output",
			input:  "",
			expect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmuxSessionList(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("len(got)=%d, want %d", len(got), len(tt.expect))
			}
			for i := range tt.expect {
				if got[i] != tt.expect[i] {
					t.Fatalf("got[%d]=%q, want %q", i, got[i], tt.expect[i])
				}
			}
		})
	}
}
