package smartcontracts

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/bindings"
)

type Payment interface {
	Account(ctx context.Context, token, owner common.Address) (*AccountInfo, error)
	GetRailsForPayeeAndToken(ctx context.Context, payee, token common.Address, offset, limit *big.Int) (*RailsResult, error)
	GetRail(ctx context.Context, railId *big.Int) (*RailView, error)

	// Address returns the payment contract address
	Address() common.Address

	// PackSettleRail returns the packed ABI call data for settleRail
	// This can be used with a Sender to submit the transaction
	PackSettleRail(railId, untilEpoch *big.Int) ([]byte, error)

	// PackWithdrawTo returns the packed ABI call data for withdrawTo
	// This can be used with a Sender to submit the transaction
	PackWithdrawTo(token, to common.Address, amount *big.Int) ([]byte, error)
}

type paymentContract struct {
	address  common.Address
	contract *bindings.Payments
	client   bind.ContractBackend
}

func NewPaymentContract(address common.Address, client bind.ContractBackend) (Payment, error) {
	contract, err := bindings.NewPayments(address, client)
	if err != nil {
		return nil, err
	}

	return &paymentContract{
		address:  address,
		contract: contract,
		client:   client,
	}, nil
}

type AccountInfo struct {
	Funds               *big.Int
	LockupCurrent       *big.Int
	LockupRate          *big.Int
	LockupLastSettledAt *big.Int
}

type RailInfo struct {
	RailId       *big.Int
	IsTerminated bool
	EndEpoch     *big.Int
}

type RailsResult struct {
	Rails      []RailInfo
	NextOffset *big.Int
	Total      *big.Int
}

type RailView struct {
	RailId              *big.Int
	Token               common.Address
	From                common.Address
	To                  common.Address
	Operator            common.Address
	Validator           common.Address
	PaymentRate         *big.Int
	LockupPeriod        *big.Int
	LockupFixed         *big.Int
	SettledUpTo         *big.Int
	EndEpoch            *big.Int
	CommissionRateBps   *big.Int
	ServiceFeeRecipient common.Address
}

func (p *paymentContract) Account(ctx context.Context, token, owner common.Address) (*AccountInfo, error) {
	info, err := p.contract.Accounts(&bind.CallOpts{Context: ctx}, token, owner)
	if err != nil {
		return nil, err
	}

	return &AccountInfo{
		Funds:               info.Funds,
		LockupCurrent:       info.LockupCurrent,
		LockupRate:          info.LockupRate,
		LockupLastSettledAt: info.LockupLastSettledAt,
	}, nil
}

func (p *paymentContract) GetRailsForPayeeAndToken(ctx context.Context, payee, token common.Address, offset, limit *big.Int) (*RailsResult, error) {
	result, err := p.contract.GetRailsForPayeeAndToken(&bind.CallOpts{Context: ctx}, payee, token, offset, limit)
	if err != nil {
		return nil, err
	}

	rails := make([]RailInfo, len(result.Results))
	for i, r := range result.Results {
		rails[i] = RailInfo{
			RailId:       r.RailId,
			IsTerminated: r.IsTerminated,
			EndEpoch:     r.EndEpoch,
		}
	}

	return &RailsResult{
		Rails:      rails,
		NextOffset: result.NextOffset,
		Total:      result.Total,
	}, nil
}

func (p *paymentContract) GetRail(ctx context.Context, railId *big.Int) (*RailView, error) {
	rail, err := p.contract.GetRail(&bind.CallOpts{Context: ctx}, railId)
	if err != nil {
		return nil, err
	}

	return &RailView{
		RailId:              railId,
		Token:               rail.Token,
		From:                rail.From,
		To:                  rail.To,
		Operator:            rail.Operator,
		Validator:           rail.Validator,
		PaymentRate:         rail.PaymentRate,
		LockupPeriod:        rail.LockupPeriod,
		LockupFixed:         rail.LockupFixed,
		SettledUpTo:         rail.SettledUpTo,
		EndEpoch:            rail.EndEpoch,
		CommissionRateBps:   rail.CommissionRateBps,
		ServiceFeeRecipient: rail.ServiceFeeRecipient,
	}, nil
}

func (p *paymentContract) Address() common.Address {
	return p.address
}

func (p *paymentContract) PackSettleRail(railId, untilEpoch *big.Int) ([]byte, error) {
	abi, err := bindings.PaymentsMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return abi.Pack("settleRail", railId, untilEpoch)
}

func (p *paymentContract) PackWithdrawTo(token, to common.Address, amount *big.Int) ([]byte, error) {
	abi, err := bindings.PaymentsMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return abi.Pack("withdrawTo", token, to, amount)
}
