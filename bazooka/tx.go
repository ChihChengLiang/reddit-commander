package bazooka

import (
	"fmt"
	big "math/big"

	"github.com/BOPR/common"
	"github.com/BOPR/config"
	"github.com/BOPR/contracts/coordinatorproxy"
	"github.com/BOPR/core"
	"github.com/ethereum/go-ethereum"
	ethCmn "github.com/ethereum/go-ethereum/common"
)

func (b *Bazooka) FireDepositFinalisation(TBreplaced core.UserAccount, siblings []core.UserAccount, subTreeHeight uint64) error {
	b.log.Info(
		"Attempting to finalise deposits",
		"NodeToBeReplaced",
		TBreplaced.String(),
		"NumberOfSiblings",
		len(siblings),
		"atDepth",
		subTreeHeight,
	)
	depositSubTreeHeight := big.NewInt(0)
	depositSubTreeHeight.SetUint64(subTreeHeight)
	var siblingData [][32]byte
	for _, sibling := range siblings {
		data, err := core.HexToByteArray(sibling.Hash)
		if err != nil {
			return err
		}
		siblingData = append(siblingData, data)
	}

	accountProof := coordinatorproxy.TypesAccountMerkleProof{}
	accountProof.AccountIP.PathToAccount = core.StringToBigInt(TBreplaced.Path)
	accountProof.Siblings = siblingData
	b.log.Debug("Account proof created", "accountProofPath", accountProof.AccountIP.PathToAccount, "siblings", accountProof.Siblings)
	data, err := b.ContractABI[common.COORDINATOR_PROXY].Pack("finaliseDepositsAndSubmitBatch", depositSubTreeHeight, accountProof)
	if err != nil {
		return err
	}
	coordinatorProxyAddr := ethCmn.HexToAddress(config.GlobalCfg.CoordinatorProxyAddress)
	// generate call msg
	callMsg := ethereum.CallMsg{
		To:   &coordinatorProxyAddr,
		Data: data,
	}
	auth, err := b.GenerateAuthObj(b.EthClient, callMsg)
	if err != nil {
		return err
	}
	b.log.Info("Broadcasting deposit finalisation transaction", "auth", auth)
	tx, err := b.CoordinatorProxy.FinaliseDepositsAndSubmitBatch(auth, depositSubTreeHeight, accountProof)
	if err != nil {
		return err
	}
	b.log.Info("Deposits successfully finalized!", "TxHash", tx.Hash())
	return nil
}

// SubmitBatch submits the batch on chain with updated root and compressed transactions
func (b *Bazooka) SubmitBatch(updatedRoot core.ByteArray, txs []core.Tx) error {
	b.log.Info(
		"Attempting to submit a new batch",
		"UpdatedRoot",
		updatedRoot.String(),
		"txs",
		len(txs),
	)

	var compressedTxs [][]byte
	for _, tx := range txs {
		compressedTx, err := tx.Compress()
		if err != nil {
			return err
		}
		compressedTxs = append(compressedTxs, compressedTx)
	}

	data, err := b.ContractABI[common.COORDINATOR_PROXY].Pack("submitBatch", compressedTxs, updatedRoot)
	if err != nil {
		return err
	}

	coordinatorProxyAddr := ethCmn.HexToAddress(config.GlobalCfg.CoordinatorProxyAddress)

	// generate call msg
	callMsg := ethereum.CallMsg{
		To:   &coordinatorProxyAddr,
		Data: data,
	}

	auth, err := b.GenerateAuthObj(b.EthClient, callMsg)
	if err != nil {
		return err
	}

	tx, err := b.CoordinatorProxy.SubmitBatch(auth, compressedTxs, updatedRoot)
	if err != nil {
		return err
	}
	fmt.Println("Sent a new batch!", tx.Hash())
	return nil
}
