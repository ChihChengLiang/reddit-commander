package poller

import (
	"github.com/BOPR/types"
	"testing"
)

func TestApplyTx(t *testing.T) {
	tx := types.NewTx(00, 01, 1, 1, "")
	dbInstance := NewDB(txe)
	aggreagator := NewAggregator()
	ApplyTx(tx)
}
