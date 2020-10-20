package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/BOPR/common"
)

type Genesis struct {
	StartEthBlock           uint64 `json:"startEthBlock"`
	MaxTreeDepth            uint64 `json:"maxTreeDepth"`
	MaxDepositSubTreeHeight uint64 `json:"maxDepositSubTreeHeight"`
	StakeAmount             uint64 `json:"stakeAmount"`
	GenesisAccountCount     uint64 `json:"genesisAccountCount"`
	// GenesisAccounts         GenesisAccounts `json:"genesisAccounts"`
}

// Validate validates the genesis file and checks for basic things
func (g Genesis) Validate() error {
	// if int(math.Exp2(float64(g.MaxTreeDepth)))-len(g.GenesisAccounts.Accounts) < 0 {
	// 	return errors.New("More accounts submitted than can be accomodated")
	// }

	// if len(g.GenesisAccounts.Accounts) < 1 {
	// 	return errors.New("Genesis file must contain atleast coordinator leaf")
	// }

	// if !g.GenesisAccounts.Accounts[0].IsCoordinator() {
	// 	return errors.New("First account in the genesis file should be the coordinator")
	// }

	return nil
}

// GenUserState exists to allow remove circular dependency with types
// and to allow storing more data about the account than the data in UserState
type GenUserState struct {
	ID        uint64 `json:"ID"`
	Balance   uint64
	TokenType uint64
	Nonce     uint64
	Status    uint64
	PublicKey string
}

func (acc *GenUserState) IsCoordinator() bool {
	if acc.ID != 0 || acc.Balance != 0 || acc.TokenType != 0 || acc.Nonce != 0 || acc.Status != 1 {
		return false
	}
	return true
}

func NewGenUserState(id, balance, tokenType, nonce, status uint64, publicKey string) GenUserState {
	return GenUserState{
		ID:        id,
		Balance:   balance,
		TokenType: tokenType,
		Nonce:     nonce,
		Status:    status,
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
	return NewGenUserState(0, 0, 0, 0, 100, "")
}

func DefaultGenesisAccounts() GenesisAccounts {
	var accounts []GenUserState

	// add coordinator account
	acc := NewGenUserState(common.COORDINATOR, common.COORDINATOR, common.COORDINATOR, common.COORDINATOR, 1, "0")
	accounts = append(accounts, acc)

	return NewGenesisAccounts(accounts)
}

func DefaultGenesis() Genesis {
	return Genesis{
		StartEthBlock:           0,
		MaxTreeDepth:            common.DEFAULT_DEPTH,
		MaxDepositSubTreeHeight: common.DEFAULT_DEPTH,
		StakeAmount:             32,
		GenesisAccountCount:     2,
		// GenesisAccounts:         DefaultGenesisAccounts(),
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
