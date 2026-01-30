package httpapi

// Logging
type (
	ListLogLevelsResponse struct {
		Loggers map[string]string `json:"loggers"`
	}
	SetLogLevelRequest struct {
		System string `json:"system"`
		Level  string `json:"level"`
	}

	SetLogLevelRegexRequest struct {
		Expression string `json:"expression"`
		Level      string `json:"level"`
	}
)

// Payment
type (
	GetAccountInfoResponse struct {
		Funds               string     `json:"funds"`
		LockupCurrent       string     `json:"lockup_current"`
		LockupRate          string     `json:"lockup_rate"`
		LockupLastSettledAt string     `json:"lockup_last_settled_at"`
		AvailableToWithdraw string     `json:"available_to_withdraw"`
		CurrentEpoch        string     `json:"current_epoch"`
		Rails               []RailView `json:"rails"`
	}

	RailView struct {
		RailID              string `json:"rail_id"`
		DataSetID           string `json:"data_set_id"`
		Token               string `json:"token"`
		From                string `json:"from"`
		To                  string `json:"to"`
		Operator            string `json:"operator"`
		Validator           string `json:"validator"`
		PaymentRate         string `json:"payment_rate"`
		LockupPeriod        string `json:"lockup_period"`
		LockupFixed         string `json:"lockup_fixed"`
		SettledUpTo         string `json:"settled_up_to"`
		EndEpoch            string `json:"end_epoch"`
		CommissionRateBps   string `json:"commission_rate_bps"`
		ServiceFeeRecipient string `json:"service_fee_recipient"`
		IsTerminated        bool   `json:"is_terminated"`
		UnsettledEpochs     string `json:"unsettled_epochs"`
		UnsettledAmount     string `json:"unsettled_amount"`
		SettleableEpochs    string `json:"settleable_epochs"`
		SettleableAmount    string `json:"settleable_amount"`    // gross amount (epochs * rate)
		NetSettleableAmount string `json:"net_settleable_amount"` // actual amount after proof validation
		CommissionFee       string `json:"commission_fee"`
	}

	EstimateSettlementResponse struct {
		RailID              string `json:"rail_id"`
		DataSetID           string `json:"data_set_id"`
		GrossSettleableAmount string `json:"gross_settleable_amount"` // before proof reduction
		NetSettleableAmount string `json:"net_settleable_amount"`   // after proof reduction
		ProofReductionPct   string `json:"proof_reduction_pct"`     // percentage reduced due to missed proofs
		NetworkFee          string `json:"network_fee"`
		GasLimit            string `json:"gas_limit"`
		GasPrice            string `json:"gas_price"`
		GasCost             string `json:"gas_cost"`
		NetAmount           string `json:"net_amount"` // final amount after all deductions
		UntilEpoch          string `json:"until_epoch"`
	}

	SettleRailResponse struct {
		TxHash string `json:"tx_hash"`
	}
)
