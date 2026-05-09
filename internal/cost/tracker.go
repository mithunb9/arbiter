package cost

import "sync"

type Tracker struct {
	mu            sync.Mutex
	totalUSD      float64
	monthlyBudget float64
}

func New(monthlyBudgetUSD float64) *Tracker {
	return &Tracker{monthlyBudget: monthlyBudgetUSD}
}

func (t *Tracker) Record(usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalUSD += usd
}

func (t *Tracker) Total() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalUSD
}

func (t *Tracker) BudgetExceeded() bool {
	if t.monthlyBudget == 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalUSD >= t.monthlyBudget
}
