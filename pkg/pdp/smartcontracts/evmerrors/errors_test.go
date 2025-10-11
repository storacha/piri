package evmerrors

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestParseRevert_InvalidEpochRange(t *testing.T) {
	// Example revert data for InvalidEpochRange(58, 7064254195)
	// Selector: 0xbb4e0af7
	// Param 1: 0x3a = 58
	// Param 2: 0x1a50ff6f3 = 7064254195
	revertData := "0xbb4e0af7" +
		"000000000000000000000000000000000000000000000000000000000000003a" + // 58
		"00000000000000000000000000000000000000000000000000000001a50ff6f3" // 7064254195

	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		t.Fatalf("Failed to parse revert data: %v", parseErr)
	}

	// Check that it's the right error type
	if !IsInvalidEpochRange(err) {
		t.Fatalf("Expected InvalidEpochRange, got %T", err)
	}

	// Cast and verify fields
	epochErr := err.(*InvalidEpochRange)
	if epochErr.FromEpoch.Uint64() != 58 {
		t.Errorf("Expected FromEpoch=58, got %v", epochErr.FromEpoch)
	}
	if epochErr.ToEpoch.Uint64() != 7064254195 {
		t.Errorf("Expected ToEpoch=7064254195, got %v", epochErr.ToEpoch)
	}

	// Verify error message
	expectedMsg := "InvalidEpochRange(FromEpoch=58, ToEpoch=7064254195)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message: %s, got: %s", expectedMsg, err.Error())
	}

	// Verify selector
	if epochErr.ErrorSelector() != "0xbb4e0af7" {
		t.Errorf("Expected selector 0xbb4e0af7, got %s", epochErr.ErrorSelector())
	}
}

func TestParseRevert_ZeroAddress(t *testing.T) {
	// ZeroAddress(field=2) - field is an enum represented as uint8
	// Selector: 0x620b9903
	revertData := "0x620b9903" +
		"0000000000000000000000000000000000000000000000000000000000000002" // field = 2 (USDFC)

	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		t.Fatalf("Failed to parse revert data: %v", parseErr)
	}

	if !IsZeroAddress(err) {
		t.Fatalf("Expected ZeroAddress, got %T", err)
	}

	zeroErr := err.(*ZeroAddress)
	if zeroErr.Field != 2 {
		t.Errorf("Expected Field=2, got %v", zeroErr.Field)
	}
}

func TestParseRevert_ProviderNotRegistered(t *testing.T) {
	// ProviderNotRegistered(0x1234567890123456789012345678901234567890)
	// Selector: 0x232cb27a
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	revertData := "0x232cb27a" +
		"0000000000000000000000001234567890123456789012345678901234567890"

	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		t.Fatalf("Failed to parse revert data: %v", parseErr)
	}

	if !IsProviderNotRegistered(err) {
		t.Fatalf("Expected ProviderNotRegistered, got %T", err)
	}

	provErr := err.(*ProviderNotRegistered)
	if provErr.Provider != testAddr {
		t.Errorf("Expected Provider=%s, got %s", testAddr.Hex(), provErr.Provider.Hex())
	}

	// Verify error message includes address in hex format
	expectedMsg := "ProviderNotRegistered(Provider=" + testAddr.Hex() + ")"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message: %s, got: %s", expectedMsg, err.Error())
	}
}

func TestParseRevert_NoParameters(t *testing.T) {
	// Test error with no parameters: MaxProvingPeriodZero()
	// Selector: 0xab9ff1e7
	revertData := "0xab9ff1e7"

	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		t.Fatalf("Failed to parse revert data: %v", parseErr)
	}

	if !IsMaxProvingPeriodZero(err) {
		t.Fatalf("Expected MaxProvingPeriodZero, got %T", err)
	}

	expectedMsg := "MaxProvingPeriodZero()"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message: %s, got: %s", expectedMsg, err.Error())
	}
}

func TestParseRevert_MultipleAddresses(t *testing.T) {
	// CallerNotPayerOrPayee(dataSetId=123, expectedPayer=0xAAA..., expectedPayee=0xBBB..., caller=0xCCC...)
	// Selector: 0x7e47554b
	dataSetId := uint64(123)
	expectedPayer := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	expectedPayee := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	caller := common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc")

	revertData := "0x7e47554b" +
		"000000000000000000000000000000000000000000000000000000000000007b" + // 123
		"000000000000000000000000aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + // expectedPayer
		"000000000000000000000000bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" + // expectedPayee
		"000000000000000000000000cccccccccccccccccccccccccccccccccccccccc" // caller

	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		t.Fatalf("Failed to parse revert data: %v", parseErr)
	}

	if !IsCallerNotPayerOrPayee(err) {
		t.Fatalf("Expected CallerNotPayerOrPayee, got %T", err)
	}

	callerErr := err.(*CallerNotPayerOrPayee)
	if callerErr.DataSetId.Uint64() != dataSetId {
		t.Errorf("Expected DataSetId=%d, got %v", dataSetId, callerErr.DataSetId)
	}
	if callerErr.ExpectedPayer != expectedPayer {
		t.Errorf("Expected ExpectedPayer=%s, got %s", expectedPayer.Hex(), callerErr.ExpectedPayer.Hex())
	}
	if callerErr.ExpectedPayee != expectedPayee {
		t.Errorf("Expected ExpectedPayee=%s, got %s", expectedPayee.Hex(), callerErr.ExpectedPayee.Hex())
	}
	if callerErr.Caller != caller {
		t.Errorf("Expected Caller=%s, got %s", caller.Hex(), callerErr.Caller.Hex())
	}
}

func TestParseRevertFromError_GethFormat(t *testing.T) {
	// Test parsing from Geth-style error message
	errMsg := "execution reverted: 0xab9ff1e7"

	err, parseErr := ParseRevertFromError(errMsg)
	if parseErr != nil {
		t.Fatalf("Failed to parse error message: %v", parseErr)
	}

	if !IsMaxProvingPeriodZero(err) {
		t.Fatalf("Expected MaxProvingPeriodZero, got %T", err)
	}
}

func TestParseRevertFromError_FVMFormat(t *testing.T) {
	// Test parsing from FVM-style error message
	errMsg := "failed to estimate gas: message execution failed (exit=[33], vm error=[0xbb4e0af7000000000000000000000000000000000000000000000000000000000000003a00000000000000000000000000000000000000000000000000000001a50ff6f3])"

	err, parseErr := ParseRevertFromError(errMsg)
	if parseErr != nil {
		t.Fatalf("Failed to parse error message: %v", parseErr)
	}

	if !IsInvalidEpochRange(err) {
		t.Fatalf("Expected InvalidEpochRange, got %T", err)
	}

	epochErr := err.(*InvalidEpochRange)
	if epochErr.FromEpoch.Uint64() != 58 {
		t.Errorf("Expected FromEpoch=58, got %v", epochErr.FromEpoch)
	}
}

func TestGetSelector(t *testing.T) {
	revertData := "0xbb4e0af7000000000000000000000000000000000000000000000000000000000000003a"

	selector, err := GetSelector(revertData)
	if err != nil {
		t.Fatalf("Failed to get selector: %v", err)
	}

	if selector != "0xbb4e0af7" {
		t.Errorf("Expected selector 0xbb4e0af7, got %s", selector)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Create a sample error
	err := &InvalidEpochRange{
		FromEpoch: big.NewInt(100),
		ToEpoch:   big.NewInt(50),
	}

	// Test GetErrorName
	if GetErrorName(err) != "InvalidEpochRange" {
		t.Errorf("Expected error name InvalidEpochRange, got %s", GetErrorName(err))
	}

	// Test GetErrorSelector
	if GetErrorSelector(err) != "0xbb4e0af7" {
		t.Errorf("Expected selector 0xbb4e0af7, got %s", GetErrorSelector(err))
	}

	// Test Is* helper
	if !IsInvalidEpochRange(err) {
		t.Error("IsInvalidEpochRange should return true")
	}

	if IsZeroAddress(err) {
		t.Error("IsZeroAddress should return false")
	}
}

func TestParseRevert_UnknownSelector(t *testing.T) {
	// Test with an unknown selector
	revertData := "0x00000000"

	_, err := ParseRevert(revertData)
	if err == nil {
		t.Fatal("Expected error for unknown selector")
	}

	// Check that error message mentions unknown selector
	if !strings.Contains(err.Error(), "unknown error selector: 0x00000000") {
		t.Errorf("Expected error to mention unknown selector, got: %s", err.Error())
	}
}

func TestParseRevert_InvalidHex(t *testing.T) {
	// Test with invalid hex data
	revertData := "0xgg123456"

	_, err := ParseRevert(revertData)
	if err == nil {
		t.Fatal("Expected error for invalid hex")
	}
}

func TestParseRevert_TooShort(t *testing.T) {
	// Test with data too short for selector
	revertData := "0x1234"

	_, err := ParseRevert(revertData)
	if err == nil {
		t.Fatal("Expected error for too-short data")
	}
}

func TestContractErrorInterface(t *testing.T) {
	// Ensure all error types implement ContractError interface
	var _ ContractError = &InvalidEpochRange{}
	var _ ContractError = &ZeroAddress{}
	var _ ContractError = &MaxProvingPeriodZero{}
	var _ ContractError = &ProviderNotRegistered{}
}

// Example: Using error parsing in practice
func ExampleParseRevert() {
	// Revert data from EVM call
	revertData := "0xbb4e0af7" +
		"000000000000000000000000000000000000000000000000000000000000003a" +
		"00000000000000000000000000000000000000000000000000000001a50ff6f3"

	// Parse the revert data
	err, parseErr := ParseRevert(revertData)
	if parseErr != nil {
		panic(parseErr)
	}

	// Check error type and handle accordingly
	switch e := err.(type) {
	case *InvalidEpochRange:
		// Handle InvalidEpochRange specifically
		println("Invalid epoch range:", e.FromEpoch.String(), "to", e.ToEpoch.String())
	case *ZeroAddress:
		println("Zero address for field:", e.Field)
	default:
		println("Unexpected error:", err.Error())
	}
}

// Example: Using error assertions
func ExampleIsInvalidEpochRange() {
	revertData := "0xbb4e0af7" +
		"000000000000000000000000000000000000000000000000000000000000003a" +
		"00000000000000000000000000000000000000000000000000000001a50ff6f3"

	err, _ := ParseRevert(revertData)

	if IsInvalidEpochRange(err) {
		epochErr := err.(*InvalidEpochRange)
		println("From epoch:", epochErr.FromEpoch.String())
		println("To epoch:", epochErr.ToEpoch.String())
	}
}
