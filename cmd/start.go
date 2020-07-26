package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"

	"github.com/BOPR/common"
	"github.com/BOPR/config"

	agg "github.com/BOPR/aggregator"
	"github.com/BOPR/core"
	"github.com/BOPR/listener"
	"github.com/BOPR/rest"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// StartCmd starts the daemon
func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Starts hubble daemon",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			// populate global config object
			ReadAndInitGlobalConfig()

			InitGlobalDBInstance()

			InitGlobalBazooka()

			logger := common.Logger.With("module", "hubble")

			//
			// Create all the required services
			//

			// create aggregator service
			aggregator := agg.NewAggregator()

			// create the syncer service
			syncer := listener.NewSyncer()

			// if no row is found then we are starting the node for the first time
			syncStatus, err := core.DBInstance.GetSyncStatus()
			if err != nil && gorm.IsRecordNotFoundError(err) {
				// read genesis file
				genesis, err := config.ReadGenesisFile()
				common.PanicIfError(err)

				// loads genesis data to the database
				LoadGenesisData(genesis)
			} else if err != nil && !gorm.IsRecordNotFoundError(err) {
				logger.Error("Error connecting to database", "error", err)
				common.PanicIfError(err)
			}

			logger.Info("Starting coordinator with sync and aggregator enabled", "lastSyncedEthBlock",
				syncStatus.LastEthBlockBigInt().String(),
				"lastSyncedBatch", syncStatus.LastBatchRecorded)

			// go routine to catch signal
			catchSignal := make(chan os.Signal, 1)
			signal.Notify(catchSignal, os.Interrupt)
			go func() {
				// sig is a ^C, handle it
				for range catchSignal {
					aggregator.Stop()
					syncer.Stop()
					core.DBInstance.Close()

					// exit
					os.Exit(1)
				}
			}()

			r := mux.NewRouter()
			r.HandleFunc("/tx", rest.TxReceiverHandler).Methods("POST")
			r.HandleFunc("/account", rest.GetAccountHandler).Methods("GET")
			http.Handle("/", r)

			if err := syncer.Start(); err != nil {
				log.Fatalln("Unable to start syncer", "error")
			}

			if err := aggregator.Start(); err != nil {
				log.Fatalln("Unable to start aggregator", "error", err)
			}
			// TODO replace this with port from config
			err = http.ListenAndServe(":3000", r)
			if err != nil {
				panic(err)
			}
			fmt.Println("Server started on port 3000 🎉")
		},
	}
}

func ReadAndInitGlobalConfig() {
	// create viper object
	viperObj := viper.New()

	// get current directory
	dir, err := os.Getwd()
	common.PanicIfError(err)

	// set config paths
	viperObj.SetConfigName(ConfigFileName) // name of config file (without extension)
	viperObj.AddConfigPath(dir)

	// finally! read config
	err = viperObj.ReadInConfig()
	common.PanicIfError(err)

	// unmarshall to the configration object
	var cfg config.Configuration
	if err = viperObj.UnmarshalExact(&cfg); err != nil {
		common.PanicIfError(err)
	}

	// init global config
	config.GlobalCfg = cfg
	// TODO use a better way to handle priv keys post testnet
	common.PanicIfError(config.SetOperatorKeys(config.GlobalCfg.OperatorKey))
}

func InitGlobalDBInstance() {
	// create db Instance
	tempDB, err := core.NewDB()
	common.PanicIfError(err)

	// init global DB instance
	core.DBInstance = tempDB
}

func InitGlobalBazooka() {
	var err error
	// create and init global config object
	core.LoadedBazooka, err = core.NewPreLoadedBazooka()
	common.PanicIfError(err)
}

// LoadGenesisData helps load the genesis data into the DB
func LoadGenesisData(genesis config.Genesis) {
	err := genesis.Validate()
	if err != nil {
		common.PanicIfError(err)
	}

	genesisAccounts, err := core.LoadedBazooka.GetGenesisAccounts()
	common.PanicIfError(err)
	zeroAccount := genesisAccounts[0]
	diff := int(math.Exp2(float64(genesis.MaxTreeDepth))) - len(genesisAccounts)
	var allAccounts []core.UserAccount
	var allPDALeaf []core.PDA

	// convert genesis accounts to user accounts
	for _, account := range genesisAccounts {
		pubkeyHash := core.ZERO_VALUE_LEAF.String()
		allPDALeaf = append(allPDALeaf, core.PDA{Hash: pubkeyHash})
		allAccounts = append(
			allAccounts,
			account,
		)
	}

	// fill the tree with zero leaves
	for diff > 0 {
		newAcc := core.EmptyAccount()
		newAcc.Data = zeroAccount.Data
		newAcc.Hash = core.ZERO_VALUE_LEAF.String()
		allAccounts = append(allAccounts, newAcc)
		newPDA := core.NewEmptyPDA()
		newPDA.Hash = core.ZERO_VALUE_LEAF.String()
		allPDALeaf = append(allPDALeaf, *newPDA)
		diff--
	}

	// load accounts
	err = core.DBInstance.InitBalancesTree(genesis.MaxTreeDepth, allAccounts)
	common.PanicIfError(err)

	err = core.DBInstance.InitPDATree(genesis.MaxTreeDepth, allPDALeaf)
	common.PanicIfError(err)

	// load params
	newParams := core.Params{StakeAmount: genesis.StakeAmount, MaxDepth: genesis.MaxTreeDepth, MaxDepositSubTreeHeight: genesis.MaxDepositSubTreeHeight}
	core.DBInstance.UpdateStakeAmount(newParams.StakeAmount)
	core.DBInstance.UpdateMaxDepth(newParams.MaxDepth)
	core.DBInstance.UpdateDepositSubTreeHeight(newParams.MaxDepositSubTreeHeight)
	core.DBInstance.UpdateFinalisationTimePerBatch(40320)

	// load sync status
	err = core.DBInstance.InitSyncStatus(genesis.StartEthBlock)
	if err != nil {
		panic(err)
	}

	// load last nonce
	nonce, err := core.LoadedBazooka.EthClient.PendingNonceAt(context.Background(), config.OperatorAddress)
	if err != nil {
		return
	}
	core.DBInstance.LogBatch("INIT", nonce-1, core.TX_GENESIS)
}
