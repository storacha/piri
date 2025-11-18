package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"github.com/storacha/filecoin-services/go/eip712"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

// TODO there are several things we should do here as a sanity check to avoid having a really bad time "debugging" shit:
// 1. Check if the provider attempting to create a proof is a. register and b. approved (we do this)
// 2. Check that the payer has deposited funds in the contract, this might be hard...
// In order for this operation to succeed the following must be true:
// 1. This node has registered with the contract
// 2. the contract owner has approved this node
// 3. the payer has authorized the service contract to act on its behalf
// 4. the payer has deposited funds into the payment channel for the service contract to use
// without these we get really unhelpful errors back *sobs*

func (p *PDPService) CreateProofSet(ctx context.Context) (res common.Hash, retErr error) {
	log.Infow("creating proof set")
	defer func() {
		if retErr != nil {
			log.Errorw("failed to create proof set", "error", retErr)
		} else {
			log.Infow("created proof set", "tx", res.String())
		}
	}()

	// Check if the provider is both registered and approved
	if err := p.RequireProviderApproved(ctx); err != nil {
		return common.Hash{}, err
	}

	// Get the next client dataset ID for this payer, each payer has their own ID, which is different from the data set ID
	nextClientDataSetId, err := p.serviceContract.GetNextClientDataSetId(ctx, smartcontracts.PayerAddress)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get next client dataset ID: %w", err)
	}
	log.Infof("Next client dataset ID for payer %s: %s", smartcontracts.PayerAddress, nextClientDataSetId)

	// TODO: limit, or remove the extra data that can be provided to this method
	// the caller of this will be the operator, we could encode a did here or something
	var metadataEntries []eip712.MetadataEntry
	// request a signature for creating the dataset from the signing service
	signature, err := p.signingService.SignCreateDataSet(ctx,
		p.id,
		nextClientDataSetId,
		p.address, // Use the nodes address as the address receiving payment for storage
		metadataEntries,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign CreateDataSet: %w", err)
	}

	// Encode the extraData with payer, metadata, and signature
	extraDataBytes, err := p.edc.EncodeCreateDataSetExtraData(
		smartcontracts.PayerAddress,
		nextClientDataSetId,
		metadataEntries,
		signature,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode extraData: %w", err)
	}

	// Obtain the ABI of the PDPVerifier contract
	abiData, err := p.verifierContract.GetABI()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get contract ABI: %w", err)
	}

	// Pack the method call data with listener address and extraData
	data, err := abiData.Pack("createDataSet", smartcontracts.Addresses().Service, extraDataBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack create proof set: %w", err)
	}

	// Prepare the transaction (nonce will be set to 0, SenderETH will assign it)
	tx := ethtypes.NewTransaction(
		0,
		smartcontracts.Addresses().Verifier,
		smartcontracts.SybilFee(),
		0,
		nil,
		data,
	)

	reason := "pdp-mkproofset"
	txHash, err := p.sender.Send(ctx, p.address, tx, reason)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgWait := models.MessageWaitsEth{
			SignedTxHash: txHash.Hex(),
			TxStatus:     "pending",
		}
		if err := tx.Create(&msgWait).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", msgWait.TableName(), err)
		}

		proofsetCreate := models.PDPProofsetCreate{
			CreateMessageHash: txHash.Hex(),
			Service:           p.name,
		}
		if err := tx.Create(&proofsetCreate).Error; err != nil {
			return fmt.Errorf("failed to insert into %s: %w", proofsetCreate.TableName(), err)
		}

		// Return nil to commit the transaction.
		return nil
	}); err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}
