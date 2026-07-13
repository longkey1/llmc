package cmd

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "full date",
			dateStr: "2026-07-13",
			want:    time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "year and month",
			dateStr: "2026-07",
			want:    time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "year only",
			dateStr: "2026",
			want:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid format",
			dateStr: "07/13/2026",
			wantErr: true,
		},
		{
			name:    "empty string",
			dateStr: "",
			wantErr: true,
		},
		{
			name:    "invalid month",
			dateStr: "2026-13",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDate(tt.dateStr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseDate(%q) = %v, want %v", tt.dateStr, got, tt.want)
			}
		})
	}
}
