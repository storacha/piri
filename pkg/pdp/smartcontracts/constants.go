package smartcontracts

import (
	"time"

	"github.com/filecoin-project/lotus/chain/types"
)

const FilecoinEpoch = 30 * time.Second

// NB: definition here: https://github.com/storacha/filecoin-services/blob/main/service_contracts/src/FilecoinWarmStorageService.sol#L23
const NumChallenges = 5

// NB: definition here: https://github.com/FilOzone/pdp/blob/main/src/Fees.sol#L11
var SybilFee = types.MustParseFIL("0.1").Int

// NB: definition here: https://github.com/storacha/filecoin-services/blob/main/service_contracts/src/ServiceProviderRegistry.sol#L54
var RegisterFee = types.MustParseFIL("5").Int
