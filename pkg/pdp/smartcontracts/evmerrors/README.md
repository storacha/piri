# EVM Error Bindings

Automatically generated Go bindings for Solidity custom errors with selector-based decoding.

## Overview

This package provides type-safe Go bindings for all custom errors defined across multiple Solidity contracts:
- `service_contracts/src/Errors.sol` (48 errors)
- `service_contracts/lib/fws-payments/src/Errors.sol` (47 errors)

It allows you to parse EVM revert data (hex strings starting with a 4-byte selector) and decode them into strongly-typed Go error structures. Duplicate errors across contracts are automatically deduplicated during generation.

## Features

- **95 unique error types** automatically generated from Solidity error definitions
- **Selector-to-decoder map** for fast error lookup and decoding
- **Type-safe error assertions** with `IsXxx()` helper functions
- **Human-readable error messages** with parameter formatting
- **Multiple error message format support** (raw hex, Geth format, FVM format)
- **Automatic deduplication** of errors across multiple contracts
- **Zero dependencies** on external code generation tools (uses standard `abigen` approach)

## Usage

### Basic Example

```go

// Revert data from an EVM contract call
revertData := "0xbb4e0af7000000000000000000000000000000000000000000000000000000000000003a..."

// Parse the revert data
err, parseErr := evmerrors.ParseRevert(revertData)
if parseErr != nil {
    // Handle parsing error
}

// Check error type and handle
if evmerrors.IsInvalidEpochRange(err) {
    epochErr := err.(*evmerrors.InvalidEpochRange)
    fmt.Printf("Invalid epoch range: %d to %d\n", epochErr.FromEpoch, epochErr.ToEpoch)
}
```

### Parsing from Error Messages

The parser can extract hex data from common error message formats:

```go
// From Geth-style error message
errMsg := "execution reverted: 0xbb4e0af7..."
err, _ := evmerrors.ParseRevertFromError(errMsg)

// From FVM-style error message
errMsg := "vm error=[0xbb4e0af7...]"
err, _ := evmerrors.ParseRevertFromError(errMsg)
```

### Type Assertions

```go
// Using type switch
switch e := err.(type) {
case *evmerrors.InvalidEpochRange:
    fmt.Printf("Invalid range: %d to %d\n", e.FromEpoch, e.ToEpoch)
case *evmerrors.ZeroAddress:
    fmt.Printf("Zero address for field: %d\n", e.Field)
case *evmerrors.ProviderNotRegistered:
    fmt.Printf("Provider not registered: %s\n", e.Provider.Hex())
default:
    fmt.Printf("Unexpected error: %s\n", err.Error())
}

// Using helper functions
if evmerrors.IsInvalidEpochRange(err) {
    // Handle InvalidEpochRange specifically
}
```

### Error Information

All errors implement the `ContractError` interface:

```go
type ContractError interface {
    error
    ErrorName() string
    ErrorSelector() string
}

// Get error information
name := evmerrors.GetErrorName(err)           // "InvalidEpochRange"
selector := evmerrors.GetErrorSelector(err)   // "0xbb4e0af7"
```

## Regenerating Bindings

After modifying either `Errors.sol` file, regenerate the bindings:

```bash
cd /path/to/filecoin-services
./tools/generate-evm-error-bindings.sh
```

This script:
1. Compiles both `Errors.sol` files (if needed)
2. Extracts error ABIs from both sources
3. Merges and deduplicates errors by signature
4. Generates Go error types (`errors.go`)
5. Generates decoder functions (`decoders.go`)
6. Generates helper functions (`helpers.go`)

## Error List

This package supports **95 unique error types** merged from:
- **48 errors** from `service_contracts/src/Errors.sol` (warm storage service errors)
- **47 errors** from `lib/fws-payments/src/Errors.sol` (payment rail errors)

<details>
<summary>Sample errors from warm storage service (click to expand)</summary>

- `CDNPaymentAlreadyTerminated(uint256 dataSetId)`
- `CacheMissPaymentAlreadyTerminated(uint256 dataSetId)`
- `ChallengeWindowTooEarly(uint256 dataSetId, uint256 windowStart, uint256 nowBlock)`
- `DataSetNotRegistered(uint256 dataSetId)`
- `DataSetPaymentBeyondEndEpoch(uint256 dataSetId, uint256 pdpEndEpoch, uint256 currentBlock)`
- `DivisionByZero()`
- `FilBeamServiceNotConfigured(uint256 dataSetId)`
- `InvalidChallengeCount(uint256 dataSetId, uint256 minExpected, uint256 actual)`
- `InvalidEpochRange(uint256 fromEpoch, uint256 toEpoch)`
- `MaxProvingPeriodZero()`
- `ProviderNotRegistered(address provider)`
- `ProvingPeriodPassed(uint256 dataSetId, uint256 deadline, uint256 nowBlock)`
- `ZeroAddress(uint8 field)`

</details>

<details>
<summary>Sample errors from payment rails (click to expand)</summary>

- `CannotSettleFutureEpochs(uint256 railId, uint256 maxAllowedEpoch, uint256 attemptedEpoch)`
- `InsufficientFundsForSettlement(address token, address from, uint256 available, uint256 required)`
- `InsufficientLockupForSettlement(address token, address from, uint256 available, uint256 required)`
- `LockupExceedsFundsInvariant(address token, address account, uint256 lockupCurrent, uint256 fundsCurrent)`
- `OperatorRateAllowanceExceeded(uint256 allowed, uint256 attemptedUsage)`
- `RailAlreadyTerminated(uint256 railId)`
- `RailNotAssociated(uint256 railId)`

</details>

## Architecture

### Generated Files

- **`errors.go`** - Error type definitions with `Error()`, `ErrorName()`, and `ErrorSelector()` methods
- **`decoders.go`** - Decoder functions and selector-to-decoder map
- **`helpers.go`** - `IsXxx()` assertion functions and utility helpers
- **`parser.go`** - (Manual) Main entry point for parsing revert data

### How It Works

1. **Error Selector Computation**: Each Solidity error has a unique 4-byte selector computed as `keccak256(ErrorSignature)[:4]`
2. **Selector Lookup**: When parsing revert data, the first 4 bytes are used to look up the decoder function
3. **ABI Decoding**: The decoder uses `go-ethereum/accounts/abi` to unpack the remaining bytes into typed parameters
4. **Type-Safe Return**: Returns a strongly-typed error struct implementing the `ContractError` interface

## Testing

```bash
cd pkg/evmerrors
go test -v
```

Tests cover:
- Error parsing with various parameter types
- Multiple error message formats
- Error assertions
- Edge cases (unknown selectors, invalid hex, etc.)

## Dependencies

- `github.com/ethereum/go-ethereum` - For ABI encoding/decoding and common types

## License

Apache-2.0 OR MIT (same as the Solidity contracts)
