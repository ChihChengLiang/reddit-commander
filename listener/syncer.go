package listener

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/BOPR/common"
	"github.com/BOPR/db"
	"github.com/BOPR/types"
	"github.com/ethereum/go-ethereum"
	ethCmn "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// Syncer to sync events from ethereum chain
type Syncer struct {
	// Base service
	BaseService

	// ABIs
	abis []*abi.ABI

	// storage client
	dbInstance db.DB

	// contract caller to interact with contracts
	contractCaller types.ContractCaller

	// header channel
	HeaderChannel chan *ethTypes.Header
	// cancel function for poll/subscription
	cancelSubscription context.CancelFunc

	// header listener subscription
	cancelHeaderProcess context.CancelFunc
}

func NewSyncer() Syncer {
	// create logger
	logger := common.Logger.With(SyncerServiceName)

	// create syncer obj
	syncerService := &Syncer{}

	// create new base service
	syncerService.BaseService = *NewBaseService(logger, SyncerServiceName, syncerService)

	contractCaller, err := types.NewContractCaller()
	if err != nil {
		panic(err)
	}

	abis := []*abi.ABI{
		&contractCaller.RollupContractABI,
		&contractCaller.MerkleTreeABI,
	}
	syncerService.abis = abis
	syncerService.contractCaller = contractCaller
	syncerService.HeaderChannel = make(chan *ethTypes.Header)
	syncerService.dbInstance, err = db.NewDB()
	if err != nil {
		panic(err)
	}

	return *syncerService
}

// OnStart starts new block subscription
func (s *Syncer) OnStart() error {
	// Always call the overridden method.
	s.BaseService.OnStart()

	// create cancellable context
	ctx, cancelSubscription := context.WithCancel(context.Background())
	s.cancelSubscription = cancelSubscription

	// create cancellable context
	headerCtx, cancelHeaderProcess := context.WithCancel(context.Background())
	s.cancelHeaderProcess = cancelHeaderProcess

	// start header process
	go s.startHeaderProcess(headerCtx)

	// subscribe to new head
	subscription, err := s.contractCaller.EthClient.SubscribeNewHead(ctx, s.HeaderChannel)
	if err != nil {
		// start go routine to poll for new header using client object
		go s.startPolling(ctx, 1*time.Second)
	} else {
		// start go routine to listen new header using subscription
		go s.startSubscription(ctx, subscription)
	}
	return nil
}

// OnStop stops all necessary go routines
func (s *Syncer) OnStop() {
	s.BaseService.OnStop() // Always call the overridden method.

	// cancel subscription if any
	s.cancelSubscription()

	// cancel header process
	s.cancelHeaderProcess()

	s.dbInstance.Close()
}

// startHeaderProcess starts header process when they get new header
func (s *Syncer) startHeaderProcess(ctx context.Context) {
	fmt.Println("starting header process")
	for {
		select {
		case newHeader := <-s.HeaderChannel:
			s.processHeader(*newHeader)
		case <-ctx.Done():
			return
		}
	}
}

// startPolling starts polling
func (s *Syncer) startPolling(ctx context.Context, pollInterval time.Duration) {
	// How often to fire the passed in function in second
	interval := pollInterval

	// Setup the ticket and the channel to signal
	// the ending of the interval
	ticker := time.NewTicker(interval)
	fmt.Println("ticker setup", ticker)

	// start listening
	for {
		select {
		case <-ticker.C:
			fmt.Println("here")
			header, err := s.contractCaller.EthClient.HeaderByNumber(ctx, nil)
			if err == nil && header != nil {
				fmt.Println("found new header", header)
				// send data to channel
				s.HeaderChannel <- header
				fmt.Println("there")
			} else {
				fmt.Println("here")
			}
		case <-ctx.Done():
			fmt.Println("here")
			ticker.Stop()
			return
		}
	}
}

func (s *Syncer) startSubscription(ctx context.Context, subscription ethereum.Subscription) {
	for {
		select {
		case err := <-subscription.Err():
			// stop service
			s.Logger.Error("Error while subscribing new blocks", "error", err)
			s.Stop()

			// cancel subscription
			s.cancelSubscription()
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Syncer) processHeader(header ethTypes.Header) {
	lastLLog := s.dbInstance.GetLastListenerLog()
	query := ethereum.FilterQuery{
		FromBlock: &lastLLog.LastRecordedBlock,
		ToBlock:   header.Number,
		Addresses: []ethCmn.Address{
			s.contractCaller.RollupContractAddress,
			s.contractCaller.MerkleTreeLibAddress,
		},
	}
	s.dbInstance.StoreListenerLog(types.ListenerLog{LastRecordedBlock: *header.Number})

	// get all logs
	logs, err := s.contractCaller.EthClient.FilterLogs(context.Background(), query)
	if err != nil {
		s.Logger.Error("Error while filtering logs from syncer", "error", err)
		return
	} else if len(logs) > 0 {
		s.Logger.Debug("New logs found", "numberOfLogs", len(logs))
	}

	// log
	for _, vLog := range logs {
		topic := vLog.Topics[0].Bytes()
		for _, abiObject := range s.abis {
			selectedEvent := EventByID(abiObject, topic)
			if selectedEvent != nil {
				s.Logger.Debug("selectedEvent ", " event name -", selectedEvent.Name)
				switch selectedEvent.Name {

				}
				break
			}
		}
	}
}

func EventByID(abiObject *abi.ABI, sigdata []byte) *abi.Event {
	for _, event := range abiObject.Events {
		if bytes.Equal(event.ID().Bytes(), sigdata) {
			return &event
		}
	}
	return nil
}
