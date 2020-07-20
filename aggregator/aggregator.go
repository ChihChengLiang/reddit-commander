package aggregator

import (
	"context"
	"fmt"
	"time"

	"github.com/BOPR/common"
	"github.com/BOPR/config"
	"github.com/BOPR/core"
)

const (
	AggregatingService = "aggregator"
)

// Aggregator is the service which is supposed to create batches
// It has the following tasks:
// 1. Pick txs from the mempool
// 2. Validate these trnsactions
// 3. Update the DB post running each tx
// 4. Finally create a batch of all the transactions and post on-chain
type Aggregator struct {
	// Base service
	core.BaseService

	// contract caller to interact with contracts
	LoadedBazooka core.Bazooka

	// DB instance
	DB core.DB

	// header listener subscription
	cancelAggregating context.CancelFunc
}

// NewAggregator returns new aggregator object
func NewAggregator() *Aggregator {
	// create logger
	logger := common.Logger.With("module", AggregatingService)
	LoadedBazooka, err := core.NewPreLoadedBazooka()
	if err != nil {
		panic(err)
	}
	aggregator := &Aggregator{}
	aggregator.BaseService = *core.NewBaseService(logger, AggregatingService, aggregator)
	DB, err := core.NewDB()
	if err != nil {
		panic(err)
	}
	aggregator.DB = DB
	aggregator.LoadedBazooka = LoadedBazooka
	return aggregator
}

// OnStart starts new block subscription
func (a *Aggregator) OnStart() error {
	a.BaseService.OnStart() // Always call the overridden method.

	ctx, cancelAggregating := context.WithCancel(context.Background())
	a.cancelAggregating = cancelAggregating

	// start polling for checkpoint in buffer
	go a.startAggregating(ctx, config.GlobalCfg.PollingInterval)
	return nil
}

// OnStop stops all necessary go routines
func (a *Aggregator) OnStop() {
	a.BaseService.OnStop() // Always call the overridden method.
	a.DB.Close()
	// cancel ack process
	a.cancelAggregating()
}

func (a *Aggregator) startAggregating(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	// stop ticker when everything done
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.pickBatch()
			// pick batch from DB
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (a *Aggregator) pickBatch() {
	txs, err := a.DB.PopTxs()
	if err != nil {
		fmt.Println("Error while popping txs from mempool", "Error", err)
	}

	// Step-2
	err = a.ProcessTx(txs)
	if err != nil {
		fmt.Println("Error while processing tx", "error", err)
		return
	}

	// Step-3
	// Finally create a merkel root of all updated leafs and push batch on-chain
	rootAcc, err := a.DB.GetRoot()
	if err != nil {
		fmt.Println("Error while getting root", "error", err)
		return
	}
	err = a.LoadedBazooka.SubmitBatch(rootAcc.HashToByteArray(), txs)
	if err != nil {
		fmt.Println("Error while submitting batch", "error", err)
		return
	}
}

// ProcessTx fetches all the data required to validate tx from smart contact
// and calls the proccess tx function to return the updated balance root and accounts
func (a *Aggregator) ProcessTx(txs []core.Tx) error {
	for _, tx := range txs {
		rootAcc, err := a.DB.GetRoot()
		if err != nil {
			return err
		}

		a.Logger.Debug("Latest root", "root", rootAcc.Hash)

		currentRoot, err := core.HexToByteArray(rootAcc.Hash)
		if err != nil {
			return err
		}

		currentAccountTreeRoot, err := core.HexToByteArray(rootAcc.PublicKeyHash)
		if err != nil {
			return err
		}

		fromAccProof, toAccProof, PDAproof, err := tx.GetVerificationData()
		if err != nil {
			a.Logger.Error("Unable to create verification data", "error", err)
			return err
		}

		a.Logger.Debug("Fetched latest account proofs", "tx", tx.String(), "fromMP", fromAccProof, "toMP", toAccProof, "PDAProof", PDAproof)

		updatedRoot, updatedFrom, updatedTo, err := a.LoadedBazooka.ProcessTx(currentRoot, currentAccountTreeRoot, tx, fromAccProof, toAccProof, PDAproof)
		if err != nil {
			err := tx.UpdateStatus(core.TX_STATUS_REVERTED)
			if err != nil {
				a.Logger.Error("Unable to update transaction status", "tx", tx.String())
				return err
			}
		}

		// if the transactions is valid, apply it
		err = tx.Apply(updatedFrom, updatedTo)
		if err != nil {
			return err
		}

		currentRoot = updatedRoot
	}
	return nil
}
