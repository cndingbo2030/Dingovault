package ai

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	t.Parallel()
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	if s := CosineSimilarity(a, b); math.Abs(float64(s)) > 1e-5 {
		t.Fatalf("orthogonal got %v want 0", s)
	}
	c := []float32{2, 0, 0}
	if s := CosineSimilarity(a, c); math.Abs(float64(s-1)) > 1e-5 {
		t.Fatalf("parallel got %v want 1", s)
	}
	if s := CosineSimilarity(a, []float32{1}); s != 0 {
		t.Fatalf("mismatch len got %v want 0", s)
	}
}
