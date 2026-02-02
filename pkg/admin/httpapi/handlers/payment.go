package handlers

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/config/app"
	ethsender "github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
)

var log = logging.Logger("admin/payment")

type PaymentHandler struct {
	payment          smartcontracts.Payment
	pdpConfig        app.PDPServiceConfig
	serviceView      smartcontracts.Service
	serviceValidator smartcontracts.ServiceValidator
	ethClient        *ethclient.Client
	sender           ethsender.Sender
	db               *gorm.DB
}

func NewPaymentHandler(payment smartcontracts.Payment, pdpConfig app.PDPServiceConfig, serviceView smartcontracts.Service, serviceValidator smartcontracts.ServiceValidator, ethClient *ethclient.Client, sender ethsender.Sender, db *gorm.DB) *PaymentHandler {
	return &PaymentHandler{
		payment:          payment,
		pdpConfig:        pdpConfig,
		serviceView:      serviceView,
		serviceValidator: serviceValidator,
		ethClient:        ethClient,
		sender:           sender,
		db:               db,
	}
}

func (h *PaymentHandler) GetAccountInfo(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	token := h.pdpConfig.Contracts.USDFCToken
	owner := h.pdpConfig.OwnerAddress

	// Get current block number (epoch)
	var currentEpoch *big.Int
	if h.ethClient != nil {
		blockNum, err := h.ethClient.BlockNumber(reqCtx)
		if err != nil {
			return ctx.String(http.StatusInternalServerError, "getting current block: "+err.Error())
		}
		currentEpoch = new(big.Int).SetUint64(blockNum)
	} else {
		currentEpoch = big.NewInt(0)
	}

	info, err := h.payment.Account(reqCtx, token, owner)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	// Calculate available to withdraw
	availableToWithdraw := new(big.Int).Sub(info.Funds, info.LockupCurrent)
	if availableToWithdraw.Sign() < 0 {
		availableToWithdraw = big.NewInt(0)
	}

	// Get rails where this owner is the payee
	railsResult, err := h.payment.GetRailsForPayeeAndToken(reqCtx, owner, token, big.NewInt(0), big.NewInt(100))
	if err != nil {
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	// Fetch full details for each rail
	rails := make([]httpapi.RailView, 0, len(railsResult.Rails))
	for _, railInfo := range railsResult.Rails {
		rail, err := h.payment.GetRail(reqCtx, railInfo.RailId)
		if err != nil {
			return ctx.String(http.StatusInternalServerError, err.Error())
		}

		// Get dataset ID for this rail
		var dataSetID string
		if h.serviceView != nil {
			dsID, err := h.serviceView.RailToDataSet(reqCtx, railInfo.RailId)
			if err == nil && dsID != nil {
				dataSetID = dsID.String()
			}
		}

		// Get payer's account to determine settleable amount
		// (settleable is capped by payer's lockupLastSettledAt)
		payerInfo, err := h.payment.Account(reqCtx, token, rail.From)
		if err != nil {
			return ctx.String(http.StatusInternalServerError, "getting payer account: "+err.Error())
		}

		// Calculate unsettled and settleable amounts (gross)
		unsettledEpochs, unsettledAmount, settleableEpochs, settleableAmount, commissionFee := h.calculateSettlement(
			rail, railInfo.IsTerminated, currentEpoch, payerInfo.LockupLastSettledAt,
		)

		// Get net settleable amount from validator (accounts for missed proofs)
		netSettleableAmount := new(big.Int).Set(settleableAmount)
		if h.serviceValidator != nil && settleableAmount.Sign() > 0 {
			// Calculate the epoch to settle up to
			untilEpoch := new(big.Int).Add(rail.SettledUpTo, settleableEpochs)
			validationResult, err := h.serviceValidator.ValidatePayment(reqCtx, railInfo.RailId, settleableAmount, rail.SettledUpTo, untilEpoch)
			if err == nil && validationResult != nil {
				netSettleableAmount = validationResult.ModifiedAmount
			}
			// If validation fails, fall back to gross amount (best effort)
		}

		rails = append(rails, httpapi.RailView{
			RailID:              rail.RailId.String(),
			DataSetID:           dataSetID,
			Token:               rail.Token.Hex(),
			From:                rail.From.Hex(),
			To:                  rail.To.Hex(),
			Operator:            rail.Operator.Hex(),
			Validator:           rail.Validator.Hex(),
			PaymentRate:         rail.PaymentRate.String(),
			LockupPeriod:        rail.LockupPeriod.String(),
			LockupFixed:         rail.LockupFixed.String(),
			SettledUpTo:         rail.SettledUpTo.String(),
			EndEpoch:            rail.EndEpoch.String(),
			CommissionRateBps:   rail.CommissionRateBps.String(),
			ServiceFeeRecipient: rail.ServiceFeeRecipient.Hex(),
			IsTerminated:        railInfo.IsTerminated,
			UnsettledEpochs:     unsettledEpochs.String(),
			UnsettledAmount:     unsettledAmount.String(),
			SettleableEpochs:    settleableEpochs.String(),
			SettleableAmount:    settleableAmount.String(),
			NetSettleableAmount: netSettleableAmount.String(),
			CommissionFee:       commissionFee.String(),
		})
	}

	return ctx.JSON(http.StatusOK, &httpapi.GetAccountInfoResponse{
		Funds:               info.Funds.String(),
		LockupCurrent:       info.LockupCurrent.String(),
		LockupRate:          info.LockupRate.String(),
		LockupLastSettledAt: info.LockupLastSettledAt.String(),
		AvailableToWithdraw: availableToWithdraw.String(),
		CurrentEpoch:        currentEpoch.String(),
		Rails:               rails,
	})
}

// calculateSettlement computes unsettled/settleable epochs and amounts for a rail
func (h *PaymentHandler) calculateSettlement(
	rail *smartcontracts.RailView,
	isTerminated bool,
	currentEpoch, lockupLastSettledAt *big.Int,
) (unsettledEpochs, unsettledAmount, settleableEpochs, settleableAmount, commissionFee *big.Int) {
	unsettledEpochs = big.NewInt(0)
	unsettledAmount = big.NewInt(0)
	settleableEpochs = big.NewInt(0)
	settleableAmount = big.NewInt(0)
	commissionFee = big.NewInt(0)

	if rail.PaymentRate.Sign() == 0 {
		return
	}

	if isTerminated && rail.EndEpoch != nil && rail.EndEpoch.Sign() > 0 {
		// Terminated rail - unsettled is up to endEpoch
		unsettledEpochs = new(big.Int).Sub(rail.EndEpoch, rail.SettledUpTo)
		// For terminated rails, streaming lockup covers all remaining epochs
		settleableEpochs = new(big.Int).Set(unsettledEpochs)
	} else {
		// Non-terminated rail
		unsettledEpochs = new(big.Int).Sub(currentEpoch, rail.SettledUpTo)

		// Settleable is capped by lockupLastSettledAt (payer's account settlement)
		capEpoch := new(big.Int).Set(currentEpoch)
		if lockupLastSettledAt.Cmp(currentEpoch) < 0 {
			capEpoch = lockupLastSettledAt
		}
		settleableEpochs = new(big.Int).Sub(capEpoch, rail.SettledUpTo)
	}

	// Clamp to zero if negative
	if unsettledEpochs.Sign() < 0 {
		unsettledEpochs = big.NewInt(0)
	}
	if settleableEpochs.Sign() < 0 {
		settleableEpochs = big.NewInt(0)
	}

	// Calculate amounts
	unsettledAmount = new(big.Int).Mul(unsettledEpochs, rail.PaymentRate)
	settleableAmount = new(big.Int).Mul(settleableEpochs, rail.PaymentRate)

	// Calculate commission fee: settleableAmount * commissionRateBps / 10000
	if rail.CommissionRateBps.Sign() > 0 && settleableAmount.Sign() > 0 {
		commissionFee = new(big.Int).Mul(settleableAmount, rail.CommissionRateBps)
		commissionFee = commissionFee.Div(commissionFee, big.NewInt(10000))
	}

	return
}

// EstimateSettlement returns estimated gas and fees for settling a rail
func (h *PaymentHandler) EstimateSettlement(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	railIDStr := ctx.Param("railId")

	railID, ok := new(big.Int).SetString(railIDStr, 10)
	if !ok {
		return ctx.String(http.StatusBadRequest, "invalid rail ID")
	}

	if h.ethClient == nil {
		return ctx.String(http.StatusServiceUnavailable, "eth client not available")
	}

	token := h.pdpConfig.Contracts.USDFCToken
	owner := h.pdpConfig.OwnerAddress

	// Get rail info
	rail, err := h.payment.GetRail(reqCtx, railID)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting rail: "+err.Error())
	}

	// Verify this is our rail (we are the payee)
	if rail.To != owner {
		return ctx.String(http.StatusForbidden, "not authorized to settle this rail")
	}

	// Get current epoch
	blockNum, err := h.ethClient.BlockNumber(reqCtx)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting current block: "+err.Error())
	}
	currentEpoch := new(big.Int).SetUint64(blockNum)

	// Get payer's account for lockup info
	payerInfo, err := h.payment.Account(reqCtx, token, rail.From)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting payer account: "+err.Error())
	}

	// Get dataset ID
	var dataSetID string
	if h.serviceView != nil {
		dsID, err := h.serviceView.RailToDataSet(reqCtx, railID)
		if err == nil && dsID != nil {
			dataSetID = dsID.String()
		}
	}

	// Calculate settleable amount and network fee
	_, _, settleableEpochs, settleableAmount, _ := h.calculateSettlement(
		rail, false, currentEpoch, payerInfo.LockupLastSettledAt,
	)

	// Network fee: ceil(amount / 200) = 0.5%
	networkFee := big.NewInt(0)
	if settleableAmount.Sign() > 0 {
		networkFee = new(big.Int).Add(settleableAmount, big.NewInt(199))
		networkFee = networkFee.Div(networkFee, big.NewInt(200))
	}

	// Calculate the epoch to settle up to
	untilEpoch := new(big.Int).Add(rail.SettledUpTo, settleableEpochs)

	// Estimate gas
	callData, err := h.payment.PackSettleRail(railID, untilEpoch)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "packing call data: "+err.Error())
	}

	contractAddr := h.payment.Address()
	gasLimit, err := h.ethClient.EstimateGas(reqCtx, ethereum.CallMsg{
		From: owner,
		To:   &contractAddr,
		Data: callData,
	})
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "estimating gas: "+err.Error())
	}

	// Get gas price
	gasPrice, err := h.ethClient.SuggestGasPrice(reqCtx)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting gas price: "+err.Error())
	}

	// Calculate gas cost in wei
	gasCost := new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPrice)

	// Get net settleable amount from validator (accounts for missed proofs)
	netSettleableAmount := new(big.Int).Set(settleableAmount)
	proofReductionPct := "0"
	if h.serviceValidator != nil && settleableAmount.Sign() > 0 {
		validationResult, err := h.serviceValidator.ValidatePayment(reqCtx, railID, settleableAmount, rail.SettledUpTo, untilEpoch)
		if err == nil && validationResult != nil {
			netSettleableAmount = validationResult.ModifiedAmount
			// Calculate reduction percentage
			if settleableAmount.Sign() > 0 {
				reduction := new(big.Int).Sub(settleableAmount, netSettleableAmount)
				// pct = (reduction * 100) / settleableAmount
				pct := new(big.Int).Mul(reduction, big.NewInt(100))
				pct = pct.Div(pct, settleableAmount)
				proofReductionPct = pct.String()
			}
		}
		// If validation fails, fall back to gross amount (best effort)
	}

	// Network fee: ceil(amount / 200) = 0.5% (applied to net amount)
	networkFee = big.NewInt(0)
	if netSettleableAmount.Sign() > 0 {
		networkFee = new(big.Int).Add(netSettleableAmount, big.NewInt(199))
		networkFee = networkFee.Div(networkFee, big.NewInt(200))
	}

	// Net amount = net settleable - network fee (gas is paid in FIL, not USDFC)
	netAmount := new(big.Int).Sub(netSettleableAmount, networkFee)
	if netAmount.Sign() < 0 {
		netAmount = big.NewInt(0)
	}

	return ctx.JSON(http.StatusOK, &httpapi.EstimateSettlementResponse{
		RailID:                railIDStr,
		DataSetID:             dataSetID,
		GrossSettleableAmount: settleableAmount.String(),
		NetSettleableAmount:   netSettleableAmount.String(),
		ProofReductionPct:     proofReductionPct,
		NetworkFee:            networkFee.String(),
		GasLimit:              fmt.Sprintf("%d", gasLimit),
		GasPrice:              gasPrice.String(),
		GasCost:               gasCost.String(),
		NetAmount:             netAmount.String(),
		UntilEpoch:            untilEpoch.String(),
	})
}

// SettleRail submits a settlement transaction
func (h *PaymentHandler) SettleRail(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	railIDStr := ctx.Param("railId")

	railID, ok := new(big.Int).SetString(railIDStr, 10)
	if !ok {
		return ctx.String(http.StatusBadRequest, "invalid rail ID")
	}

	if h.sender == nil {
		return ctx.String(http.StatusServiceUnavailable, "sender not available")
	}

	if h.ethClient == nil {
		return ctx.String(http.StatusServiceUnavailable, "eth client not available")
	}

	token := h.pdpConfig.Contracts.USDFCToken
	owner := h.pdpConfig.OwnerAddress

	// Check for pending settlement (if db is available)
	if h.db != nil {
		var pending models.RailSettlementWaits
		err := h.db.Where("rail_id = ?", railIDStr).First(&pending).Error
		if err == nil {
			// Check if the tx is still pending
			var msgWait models.MessageWaitsEth
			err := h.db.Where("signed_tx_hash = ?", pending.SignedTxHash).First(&msgWait).Error
			if err == nil && msgWait.TxStatus == "pending" {
				return ctx.JSON(http.StatusConflict, &httpapi.SettleRailResponse{
					TxHash: pending.SignedTxHash,
					Status: "pending",
					Error:  "settlement already in progress",
				})
			}
			// If confirmed/failed, delete the old record
			h.db.Delete(&pending)
		}
	}

	// Get rail info
	rail, err := h.payment.GetRail(reqCtx, railID)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting rail: "+err.Error())
	}

	// Verify this is our rail (we are the payee)
	if rail.To != owner {
		return ctx.String(http.StatusForbidden, "not authorized to settle this rail")
	}

	// Get current epoch
	blockNum, err := h.ethClient.BlockNumber(reqCtx)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting current block: "+err.Error())
	}
	currentEpoch := new(big.Int).SetUint64(blockNum)

	// Get payer's account for lockup info
	payerInfo, err := h.payment.Account(reqCtx, token, rail.From)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting payer account: "+err.Error())
	}

	// Calculate settleable epochs
	_, _, settleableEpochs, settleableAmount, _ := h.calculateSettlement(
		rail, false, currentEpoch, payerInfo.LockupLastSettledAt,
	)

	if settleableAmount.Sign() == 0 {
		return ctx.String(http.StatusBadRequest, "nothing to settle")
	}

	// Calculate the epoch to settle up to
	untilEpoch := new(big.Int).Add(rail.SettledUpTo, settleableEpochs)

	// Pack the call data
	callData, err := h.payment.PackSettleRail(railID, untilEpoch)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "packing call data: "+err.Error())
	}

	// Create transaction (nonce and gas will be filled by sender)
	contractAddr := h.payment.Address()
	tx := ethtypes.NewTransaction(
		0,             // nonce - will be set by sender
		contractAddr,  // to
		big.NewInt(0), // value
		0,             // gas limit - will be estimated by sender
		nil,           // gas price - will be set by sender
		callData,
	)

	// Send transaction
	txHash, err := h.sender.Send(reqCtx, owner, tx, fmt.Sprintf("settle_rail_%s", railIDStr))
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "sending transaction: "+err.Error())
	}

	// Insert into tracking tables (if db is available)
	if h.db != nil {
		if err := h.db.Transaction(func(txdb *gorm.DB) error {
			msgWait := models.MessageWaitsEth{
				SignedTxHash: txHash.Hex(),
				TxStatus:     "pending",
			}
			if err := txdb.Create(&msgWait).Error; err != nil {
				return err
			}

			railWait := models.RailSettlementWaits{
				RailID:       railIDStr,
				SignedTxHash: txHash.Hex(),
				CreatedAt:    time.Now(),
			}
			if err := txdb.Create(&railWait).Error; err != nil {
				return err
			}
			return nil
		}); err != nil {
			// Log but don't fail - tx was sent, just not tracked
			log.Errorw("failed to insert settlement tracking", "error", err, "txHash", txHash)
		}
	}

	return ctx.JSON(http.StatusOK, &httpapi.SettleRailResponse{
		TxHash: txHash.Hex(),
		Status: "pending",
	})
}

// GetSettlementStatus returns the status of a pending settlement for a rail
func (h *PaymentHandler) GetSettlementStatus(ctx echo.Context) error {
	railIDStr := ctx.Param("railId")

	if h.db == nil {
		return ctx.JSON(http.StatusOK, &httpapi.SettlementStatusResponse{
			RailID: railIDStr,
			Status: "none",
		})
	}

	var railWait models.RailSettlementWaits
	err := h.db.Where("rail_id = ?", railIDStr).First(&railWait).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusOK, &httpapi.SettlementStatusResponse{
				RailID: railIDStr,
				Status: "none",
			})
		}
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	var msgWait models.MessageWaitsEth
	err = h.db.Where("signed_tx_hash = ?", railWait.SignedTxHash).First(&msgWait).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// MessageWait not found - clean up orphaned rail wait
			h.db.Delete(&railWait)
			return ctx.JSON(http.StatusOK, &httpapi.SettlementStatusResponse{
				RailID: railIDStr,
				Status: "none",
			})
		}
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	resp := &httpapi.SettlementStatusResponse{
		RailID: railIDStr,
		TxHash: railWait.SignedTxHash,
		Status: msgWait.TxStatus,
	}

	if msgWait.TxSuccess != nil {
		resp.Success = *msgWait.TxSuccess
	}
	if msgWait.ConfirmedBlockNumber != nil {
		resp.ConfirmedBlock = fmt.Sprintf("%d", *msgWait.ConfirmedBlockNumber)
	}

	// Clean up if confirmed
	if msgWait.TxStatus == "confirmed" {
		h.db.Delete(&railWait)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// EstimateWithdraw returns estimated gas for a withdrawal
func (h *PaymentHandler) EstimateWithdraw(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	if h.ethClient == nil {
		return ctx.String(http.StatusServiceUnavailable, "eth client not available")
	}

	token := h.pdpConfig.Contracts.USDFCToken
	owner := h.pdpConfig.OwnerAddress

	// Parse optional request body
	var req httpapi.EstimateWithdrawRequest
	_ = ctx.Bind(&req) // Ignore error - fields are optional

	// Determine recipient (default to owner)
	recipient := owner
	if req.Recipient != "" {
		if !isValidAddress(req.Recipient) {
			return ctx.String(http.StatusBadRequest, "invalid recipient address")
		}
		recipient = ethcommon.HexToAddress(req.Recipient)
	}

	// Get account info for available balance
	info, err := h.payment.Account(reqCtx, token, owner)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting account info: "+err.Error())
	}

	// Calculate available to withdraw
	availableToWithdraw := new(big.Int).Sub(info.Funds, info.LockupCurrent)
	if availableToWithdraw.Sign() < 0 {
		availableToWithdraw = big.NewInt(0)
	}

	// Determine amount (default to max available)
	amount := availableToWithdraw
	if req.Amount != "" {
		var ok bool
		amount, ok = new(big.Int).SetString(req.Amount, 10)
		if !ok {
			return ctx.String(http.StatusBadRequest, "invalid amount")
		}
		if amount.Cmp(availableToWithdraw) > 0 {
			return ctx.String(http.StatusBadRequest, "amount exceeds available balance")
		}
	}

	if amount.Sign() == 0 {
		return ctx.String(http.StatusBadRequest, "nothing to withdraw")
	}

	// Estimate gas
	callData, err := h.payment.PackWithdrawTo(token, recipient, amount)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "packing call data: "+err.Error())
	}

	contractAddr := h.payment.Address()
	gasLimit, err := h.ethClient.EstimateGas(reqCtx, ethereum.CallMsg{
		From: owner,
		To:   &contractAddr,
		Data: callData,
	})
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "estimating gas: "+err.Error())
	}

	// Get gas price
	gasPrice, err := h.ethClient.SuggestGasPrice(reqCtx)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting gas price: "+err.Error())
	}

	// Calculate gas cost in wei
	gasCost := new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPrice)

	return ctx.JSON(http.StatusOK, &httpapi.EstimateWithdrawResponse{
		AvailableToWithdraw: availableToWithdraw.String(),
		WithdrawAmount:      amount.String(),
		Recipient:           recipient.Hex(),
		GasLimit:            fmt.Sprintf("%d", gasLimit),
		GasPrice:            gasPrice.String(),
		GasCost:             gasCost.String(),
	})
}

// Withdraw submits a withdrawal transaction
func (h *PaymentHandler) Withdraw(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	if h.sender == nil {
		return ctx.String(http.StatusServiceUnavailable, "sender not available")
	}

	if h.ethClient == nil {
		return ctx.String(http.StatusServiceUnavailable, "eth client not available")
	}

	token := h.pdpConfig.Contracts.USDFCToken
	owner := h.pdpConfig.OwnerAddress

	// Check for pending withdrawal (if db is available)
	if h.db != nil {
		var pending models.WithdrawalWaits
		err := h.db.Order("created_at DESC").First(&pending).Error
		if err == nil {
			// Check if the tx is still pending
			var msgWait models.MessageWaitsEth
			err := h.db.Where("signed_tx_hash = ?", pending.SignedTxHash).First(&msgWait).Error
			if err == nil && msgWait.TxStatus == "pending" {
				return ctx.JSON(http.StatusConflict, &httpapi.WithdrawResponse{
					TxHash: pending.SignedTxHash,
					Status: "pending",
					Error:  "withdrawal already in progress",
				})
			}
			// If confirmed/failed, delete the old record
			h.db.Delete(&pending)
		}
	}

	// Parse request body
	var req httpapi.WithdrawRequest
	_ = ctx.Bind(&req) // Ignore error - fields are optional

	// Determine recipient (default to owner)
	recipient := owner
	if req.Recipient != "" {
		if !isValidAddress(req.Recipient) {
			return ctx.String(http.StatusBadRequest, "invalid recipient address")
		}
		recipient = ethcommon.HexToAddress(req.Recipient)
	}

	// Get account info for available balance
	info, err := h.payment.Account(reqCtx, token, owner)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "getting account info: "+err.Error())
	}

	// Calculate available to withdraw
	availableToWithdraw := new(big.Int).Sub(info.Funds, info.LockupCurrent)
	if availableToWithdraw.Sign() < 0 {
		availableToWithdraw = big.NewInt(0)
	}

	// Determine amount (default to max available)
	amount := availableToWithdraw
	if req.Amount != "" {
		var ok bool
		amount, ok = new(big.Int).SetString(req.Amount, 10)
		if !ok {
			return ctx.String(http.StatusBadRequest, "invalid amount")
		}
		if amount.Cmp(availableToWithdraw) > 0 {
			return ctx.String(http.StatusBadRequest, "amount exceeds available balance")
		}
	}

	if amount.Sign() == 0 {
		return ctx.String(http.StatusBadRequest, "nothing to withdraw")
	}

	// Pack the call data
	callData, err := h.payment.PackWithdrawTo(token, recipient, amount)
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "packing call data: "+err.Error())
	}

	// Create transaction (nonce and gas will be filled by sender)
	contractAddr := h.payment.Address()
	tx := ethtypes.NewTransaction(
		0,             // nonce - will be set by sender
		contractAddr,  // to
		big.NewInt(0), // value
		0,             // gas limit - will be estimated by sender
		nil,           // gas price - will be set by sender
		callData,
	)

	// Send transaction
	txHash, err := h.sender.Send(reqCtx, owner, tx, "withdraw")
	if err != nil {
		return ctx.String(http.StatusInternalServerError, "sending transaction: "+err.Error())
	}

	// Insert into tracking tables (if db is available)
	if h.db != nil {
		if err := h.db.Transaction(func(txdb *gorm.DB) error {
			msgWait := models.MessageWaitsEth{
				SignedTxHash: txHash.Hex(),
				TxStatus:     "pending",
			}
			if err := txdb.Create(&msgWait).Error; err != nil {
				return err
			}

			withdrawWait := models.WithdrawalWaits{
				SignedTxHash: txHash.Hex(),
				CreatedAt:    time.Now(),
			}
			if err := txdb.Create(&withdrawWait).Error; err != nil {
				return err
			}
			return nil
		}); err != nil {
			// Log but don't fail - tx was sent, just not tracked
			log.Errorw("failed to insert withdrawal tracking", "error", err, "txHash", txHash)
		}
	}

	return ctx.JSON(http.StatusOK, &httpapi.WithdrawResponse{
		TxHash: txHash.Hex(),
		Status: "pending",
	})
}

// GetWithdrawalStatus returns the status of a pending withdrawal
func (h *PaymentHandler) GetWithdrawalStatus(ctx echo.Context) error {
	if h.db == nil {
		return ctx.JSON(http.StatusOK, &httpapi.WithdrawalStatusResponse{
			Status: "none",
		})
	}

	var withdrawWait models.WithdrawalWaits
	err := h.db.Order("created_at DESC").First(&withdrawWait).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusOK, &httpapi.WithdrawalStatusResponse{
				Status: "none",
			})
		}
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	var msgWait models.MessageWaitsEth
	err = h.db.Where("signed_tx_hash = ?", withdrawWait.SignedTxHash).First(&msgWait).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// MessageWait not found - clean up orphaned withdraw wait
			h.db.Delete(&withdrawWait)
			return ctx.JSON(http.StatusOK, &httpapi.WithdrawalStatusResponse{
				Status: "none",
			})
		}
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	resp := &httpapi.WithdrawalStatusResponse{
		TxHash: withdrawWait.SignedTxHash,
		Status: msgWait.TxStatus,
	}

	if msgWait.TxSuccess != nil {
		resp.Success = *msgWait.TxSuccess
	}
	if msgWait.ConfirmedBlockNumber != nil {
		resp.ConfirmedBlock = fmt.Sprintf("%d", *msgWait.ConfirmedBlockNumber)
	}

	// Clean up if confirmed
	if msgWait.TxStatus == "confirmed" {
		h.db.Delete(&withdrawWait)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// isValidAddress checks if the given string is a valid Ethereum address
func isValidAddress(addr string) bool {
	if len(addr) != 42 {
		return false
	}
	if addr[:2] != "0x" && addr[:2] != "0X" {
		return false
	}
	return true
}
