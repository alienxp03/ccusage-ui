package app

import "testing"

func TestNormalizeReportUsesDateWeekMonthAsPeriod(t *testing.T) {
	tests := []struct {
		name   string
		report string
		json   string
		want   string
	}{
		{
			name:   "daily date",
			report: "daily",
			json:   `{"daily":[{"date":"2026-06-22","totalTokens":10}],"totals":{}}`,
			want:   "2026-06-22",
		},
		{
			name:   "weekly start date",
			report: "weekly",
			json:   `{"weekly":[{"week":"2026-06-21","totalTokens":10}],"totals":{}}`,
			want:   "2026-06-21",
		},
		{
			name:   "monthly month",
			report: "monthly",
			json:   `{"monthly":[{"month":"2026-06","totalTokens":10}],"totals":{}}`,
			want:   "2026-06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, _, err := normalizeReport(tt.report, []byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("expected one row, got %d", len(rows))
			}
			if rows[0].Period != tt.want {
				t.Fatalf("expected period %q, got %q", tt.want, rows[0].Period)
			}
		})
	}
}
