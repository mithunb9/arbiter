package cost

import (
	"sync"
	"testing"
)

func TestRecord_And_Total(t *testing.T) {
	tr := New(0)
	tr.Record(1.50)
	tr.Record(0.25)
	if got := tr.Total(); got != 1.75 {
		t.Errorf("Total: got %v, want 1.75", got)
	}
}

func TestBudgetExceeded_NoBudget(t *testing.T) {
	tr := New(0)
	tr.Record(9999)
	if tr.BudgetExceeded() {
		t.Error("BudgetExceeded should be false when no budget set")
	}
}

func TestBudgetExceeded_UnderBudget(t *testing.T) {
	tr := New(10.00)
	tr.Record(9.99)
	if tr.BudgetExceeded() {
		t.Error("BudgetExceeded should be false when under budget")
	}
}

func TestBudgetExceeded_AtBudget(t *testing.T) {
	tr := New(10.00)
	tr.Record(10.00)
	if !tr.BudgetExceeded() {
		t.Error("BudgetExceeded should be true when at budget")
	}
}

func TestBudgetExceeded_OverBudget(t *testing.T) {
	tr := New(10.00)
	tr.Record(10.01)
	if !tr.BudgetExceeded() {
		t.Error("BudgetExceeded should be true when over budget")
	}
}

func TestRecord_Concurrent(t *testing.T) {
	tr := New(0)
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.Record(0.01)
		}()
	}
	wg.Wait()
	if got := tr.Total(); got < 0.9999 || got > 1.0001 {
		t.Errorf("Total after concurrent records: got %v, want ~1.00", got)
	}
}
