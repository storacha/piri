package smartcontracts

import (
	"fmt"
	"math/big"
)

const (
	capServiceURL             = "serviceURL"
	capMinPieceSizeInBytes    = "minPieceSizeInBytes"
	capMaxPieceSizeInBytes    = "maxPieceSizeInBytes"
	capStoragePricePerTibDay  = "storagePricePerTibPerDay"
	capMinProvingPeriodEpochs = "minProvingPeriodInEpochs"
	capLocation               = "location"
	capPaymentTokenAddress    = "paymentTokenAddress"
	capIpniPiece              = "ipniPiece"
	capIpniIpfs               = "ipniIpfs"
)

// BuildPDPCapabilities flattens the strongly typed PDP offering into the capability
// arrays the ServiceProviderRegistry contract expects. The registry enforces the PDP
// schema defined by REQUIRED_PDP_KEYS in ServiceProviderRegistry.sol (see
// _validateProductKeys), so every registration must include the mandatory fields
// that this helper always emits.
func BuildPDPCapabilities(offering ServiceProviderRegistryStoragePDPOffering) ([]string, [][]byte, error) {
	requiredUintPairs := []struct {
		name  string
		value *big.Int
	}{
		{name: capMinPieceSizeInBytes, value: offering.MinPieceSizeInBytes},
		{name: capMaxPieceSizeInBytes, value: offering.MaxPieceSizeInBytes},
		{name: capStoragePricePerTibDay, value: offering.StoragePricePerTibPerDay},
		{name: capMinProvingPeriodEpochs, value: offering.MinProvingPeriodInEpochs},
	}

	keys := []string{
		capServiceURL,
		capMinPieceSizeInBytes,
		capMaxPieceSizeInBytes,
		capStoragePricePerTibDay,
		capMinProvingPeriodEpochs,
		capLocation,
		capPaymentTokenAddress,
	}

	values := make([][]byte, len(keys))
	values[0] = []byte(offering.ServiceURL)
	values[5] = []byte(offering.Location)
	values[6] = offering.PaymentTokenAddress.Bytes()

	for idx, pair := range requiredUintPairs {
		encoded, err := encodeUintToBigEndian(pair.value)
		if err != nil {
			return nil, nil, fmt.Errorf("capability %s: %w", pair.name, err)
		}
		values[idx+1] = encoded
	}

	if offering.IpniPiece {
		keys = append(keys, capIpniPiece)
		values = append(values, []byte{1})
	}

	if offering.IpniIpfs {
		keys = append(keys, capIpniIpfs)
		values = append(values, []byte{1})
	}

	return keys, values, nil
}

func encodeUintToBigEndian(value *big.Int) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("value is required")
	}

	if value.Sign() < 0 {
		return nil, fmt.Errorf("value must be non-negative")
	}

	// big.Int.Bytes() already returns the minimal big-endian representation.
	buf := value.Bytes()
	if len(buf) == 0 {
		return []byte{0}, nil
	}
	return buf, nil
}
