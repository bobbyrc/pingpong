package collector

import (
	"math"
	"testing"
)

func TestGradeBufferbloat(t *testing.T) {
	tests := []struct {
		increase float64
		want     string
	}{
		{0, "A+"},
		{4.9, "A+"},
		{5, "A"},
		{29.9, "A"},
		{30, "B"},
		{59.9, "B"},
		{60, "C"},
		{199.9, "C"},
		{200, "D"},
		{399.9, "D"},
		{400, "F"},
		{1000, "F"},
	}

	for _, tt := range tests {
		got := GradeBufferbloat(tt.increase)
		if got != tt.want {
			t.Errorf("GradeBufferbloat(%.1f) = %s, want %s", tt.increase, got, tt.want)
		}
	}
}

func TestGradeToNumeric(t *testing.T) {
	tests := []struct {
		grade string
		want  float64
	}{
		{"A+", 6},
		{"A", 5},
		{"B", 4},
		{"C", 3},
		{"D", 2},
		{"F", 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := GradeToNumeric(tt.grade)
		if got != tt.want {
			t.Errorf("GradeToNumeric(%q) = %.0f, want %.0f", tt.grade, got, tt.want)
		}
	}
}

func TestComputeMedian(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5.0}, 5.0},
		{"odd count", []float64{1, 3, 2}, 2.0},
		{"even count", []float64{1, 2, 3, 4}, 2.5},
		{"already sorted", []float64{10, 20, 30, 40, 50}, 30.0},
		{"reverse sorted", []float64{50, 40, 30, 20, 10}, 30.0},
		{"duplicates", []float64{5, 5, 5}, 5.0},
	}

	for _, tt := range tests {
		got := ComputeMedian(tt.values)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("ComputeMedian(%s) = %.3f, want %.3f", tt.name, got, tt.want)
		}
	}
}

func TestComputeMedian_DoesNotMutateInput(t *testing.T) {
	input := []float64{5, 1, 3}
	ComputeMedian(input)
	// Original slice should be unchanged
	if input[0] != 5 || input[1] != 1 || input[2] != 3 {
		t.Fatalf("ComputeMedian mutated input: %v", input)
	}
}

func TestBufferbloatResult_Calculation(t *testing.T) {
	idle := 10.0
	loaded := 210.0
	increase := loaded - idle

	grade := GradeBufferbloat(increase)
	if grade != "D" {
		t.Fatalf("expected grade D for 200ms increase, got %s", grade)
	}

	numeric := GradeToNumeric(grade)
	if numeric != 2 {
		t.Fatalf("expected numeric 2 for grade D, got %.0f", numeric)
	}
}
