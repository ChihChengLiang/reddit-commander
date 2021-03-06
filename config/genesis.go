package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"os"

	"github.com/BOPR/common"
)

// Genesis describes the fields in genesis.json file
type Genesis struct {
	StartEthBlock           uint64          `json:"start_eth_block,omitempty"`     // should be set to the eth block num the contracts were deployed at
	MaxTreeDepth            uint64          `json:"max_tree_depth,omitempty"`      // height for all trees initially
	MaxDepositSubTreeHeight uint64          `json:"max_deposit_subtree,omitempty"` // max height for deposit subtrees initially
	StakeAmount             uint64          `json:"stake_amount,omitempty"`        // initial stake amount set on contracts
	GenesisAccounts         GenesisAccounts `json:"genesis_accounts,omitempty"`    // genesis accounts
}

// Validate validates the genesis file and checks for basic things
func (g Genesis) Validate() error {
	if int(math.Exp2(float64(g.MaxTreeDepth)))-len(g.GenesisAccounts.Accounts) < 0 {
		return errors.New("More accounts submitted than can be accomodated")
	}

	if len(g.GenesisAccounts.Accounts) < 1 {
		return errors.New("Genesis file must contain atleast coordinator leaf")
	}

	if !g.GenesisAccounts.Accounts[0].IsCoordinator() {
		return errors.New("First account in the genesis file should be the coordinator")
	}

	return nil
}

// GenUserState exists to allow remove circular dependency with types
// and to allow storing more data about the account than the data in UserState
type GenUserState struct {
	AccountID uint64 `json:"account_id,omitempty"`
	Nonce     uint64 `json:"nonce,omitempty"`
	Balance   uint64 `json:"balance,omitempty"`
	TokenType uint64 `json:"token_type,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

func (acc *GenUserState) IsCoordinator() bool {
	if acc.AccountID != 0 || acc.Balance != 0 || acc.TokenType != 0 || acc.Nonce != 0 {
		return false
	}
	return true
}

func NewGenUserState(_accountID, balance, tokenType, nonce uint64, publicKey string) GenUserState {
	return GenUserState{
		AccountID: _accountID,
		Balance:   balance,
		TokenType: tokenType,
		Nonce:     nonce,
		PublicKey: publicKey,
	}
}

type GenesisAccounts struct {
	Accounts []GenUserState `json:"gen_accounts"`
}

func NewGenesisAccounts(accounts []GenUserState) GenesisAccounts {
	return GenesisAccounts{Accounts: accounts}
}

func EmptyGenesisAccount() GenUserState {
	return NewGenUserState(0, 0, 0, 0, "")
}

func DefaultGenesisAccounts() GenesisAccounts {
	var accounts []GenUserState

	// add coordinator accounts
	acc1 := NewGenUserState(0, common.ZERO_UINT, common.ZERO_UINT, common.ZERO_UINT, "0")
	acc2 := NewGenUserState(1, common.ZERO_UINT, common.ZERO_UINT, common.ZERO_UINT, "0")
	accounts = append(accounts, acc1, acc2)

	return NewGenesisAccounts(accounts)
}

func DefaultGenesis() Genesis {
	return Genesis{
		StartEthBlock:           0,
		MaxTreeDepth:            common.DEFAULT_DEPTH,
		MaxDepositSubTreeHeight: common.DEFAULT_DEPTH,
		StakeAmount:             32,
		GenesisAccounts:         DefaultGenesisAccounts(),
	}
}

func ReadGenesisFile() (Genesis, error) {
	var genesis Genesis

	genesisFile, err := os.Open("genesis.json")
	if err != nil {
		return genesis, err
	}
	defer genesisFile.Close()

	genBytes, err := ioutil.ReadAll(genesisFile)
	if err != nil {
		return genesis, err
	}

	err = json.Unmarshal(genBytes, &genesis)
	return genesis, err
}

func WriteGenesisFile(genesis Genesis) error {
	bz, err := json.MarshalIndent(genesis, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile("genesis.json", bz, 0644)
}
