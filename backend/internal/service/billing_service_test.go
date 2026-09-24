package service

import (
	"testing"
	"time"

	"cylawcase/internal/model"
)

func TestInvoiceAmount(t *testing.T) {
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name    string
		entries []model.TimeEntry
		want    float64
	}{
		{name: "empty", entries: nil, want: 0},
		{
			name: "single entry one hour",
			entries: []model.TimeEntry{
				{WorkDate: d, DurationMin: 60, HourlyRate: 600},
			},
			want: 600,
		},
		{
			name: "multiple entries keep snapshot rate",
			entries: []model.TimeEntry{
				{WorkDate: d, DurationMin: 120, HourlyRate: 600}, // 1200
				{WorkDate: d, DurationMin: 90, HourlyRate: 300},  // 450
				{WorkDate: d, DurationMin: 30, HourlyRate: 0},    // 0
			},
			want: 1650,
		},
		{
			name: "old rate snapshot unaffected by later rate",
			entries: []model.TimeEntry{
				// 即便律师现在费率已涨到 800，旧工时仍按登记时的 500 计算。
				{WorkDate: d, DurationMin: 90, HourlyRate: 500}, // 750
			},
			want: 750,
		},
		{
			name: "fractional minutes rounded to 2 decimals",
			entries: []model.TimeEntry{
				{WorkDate: d, DurationMin: 25, HourlyRate: 600}, // 250
				{WorkDate: d, DurationMin: 10, HourlyRate: 100}, // 16.666...
			},
			want: 266.67,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := invoiceAmount(tc.entries); got != tc.want {
				t.Errorf("invoiceAmount() = %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{1.006, 1.01},
		{1.004, 1},
		{266.666, 266.67},
	}
	for _, tc := range cases {
		if got := round2(tc.in); got != tc.want {
			t.Errorf("round2(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
