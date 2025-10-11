package evmerrors

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ParseRevert parses EVM revert data and returns a typed ContractError
//
// The hexData parameter should be a hex string with the format:
// "0x" + 8-character selector + encoded parameters
//
// Example: "0x6c577bf9000000000000000000000000000000000000000000000000000000000000003a..."
//
// Returns:
//   - ContractError: The decoded error with typed fields
//   - error: Any parsing error that occurred
func ParseRevert(hexData string) (ContractError, error) {
	// Validate and clean input
	hexData = strings.TrimSpace(hexData)
	if hexData == "" {
		return nil, fmt.Errorf("empty revert data")
	}

	// Remove 0x prefix if present
	if strings.HasPrefix(hexData, "0x") || strings.HasPrefix(hexData, "0X") {
		hexData = hexData[2:]
	}

	// Validate minimum length (4 bytes = 8 hex characters for selector)
	if len(hexData) < 8 {
		return nil, fmt.Errorf("revert data too short: expected at least 8 hex chars for selector, got %d", len(hexData))
	}

	// Extract selector (first 4 bytes = 8 hex characters)
	selectorHex := "0x" + hexData[:8]

	// Extract parameter data (remaining bytes)
	var paramData []byte
	if len(hexData) > 8 {
		var err error
		paramData, err = hex.DecodeString(hexData[8:])
		if err != nil {
			return nil, fmt.Errorf("invalid hex in parameter data: %w", err)
		}
	}

	// Look up decoder function
	decoder, ok := ErrorDecoders[selectorHex]
	if !ok {
		return nil, fmt.Errorf("unknown error selector: %s (possible new error type or different contract)", selectorHex)
	}

	// Decode the error
	return decoder(paramData)
}

// ParseRevertFromError extracts and parses revert data from common error messages
//
// This function handles error messages in various formats:
//   - Raw hex: "0x6c577bf9..."
//   - Geth format: "execution reverted: 0x6c577bf9..."
//   - FVM format: "vm error=[0x6c577bf9...]"
//
// Returns:
//   - ContractError: The decoded error
//   - error: Any parsing error
func ParseRevertFromError(errMsg string) (ContractError, error) {
	// Extract hex data from common error message patterns
	hexData := extractHexData(errMsg)
	if hexData == "" {
		return nil, fmt.Errorf("no revert data found in error message: %s", errMsg)
	}

	return ParseRevert(hexData)
}

// extractHexData extracts hex data from common error message formats
func extractHexData(errMsg string) string {
	errMsg = strings.TrimSpace(errMsg)

	// Pattern 1: Raw hex data
	if strings.HasPrefix(errMsg, "0x") {
		return errMsg
	}

	// Pattern 2: "execution reverted: 0x..."
	if idx := strings.Index(errMsg, "execution reverted:"); idx != -1 {
		remaining := errMsg[idx+len("execution reverted:"):]
		remaining = strings.TrimSpace(remaining)
		if strings.HasPrefix(remaining, "0x") {
			return extractFirstHexString(remaining)
		}
	}

	// Pattern 3: "vm error=[0x...]"
	if idx := strings.Index(errMsg, "vm error=["); idx != -1 {
		start := idx + len("vm error=[")
		remaining := errMsg[start:]
		if strings.HasPrefix(remaining, "0x") {
			return extractFirstHexString(remaining)
		}
	}

	// Pattern 4: "revert 0x..."
	if idx := strings.Index(errMsg, "revert "); idx != -1 {
		remaining := errMsg[idx+len("revert "):]
		remaining = strings.TrimSpace(remaining)
		if strings.HasPrefix(remaining, "0x") {
			return extractFirstHexString(remaining)
		}
	}

	// Pattern 5: Look for any 0x... hex string in the message
	if idx := strings.Index(errMsg, "0x"); idx != -1 {
		return extractFirstHexString(errMsg[idx:])
	}

	return ""
}

// extractFirstHexString extracts the first valid hex string from input
func extractFirstHexString(s string) string {
	if !strings.HasPrefix(s, "0x") {
		return ""
	}

	// Find the end of the hex string (first non-hex character after 0x)
	end := 2
	for end < len(s) {
		c := s[end]
		if !isHexChar(c) {
			break
		}
		end++
	}

	return s[:end]
}

// isHexChar checks if a character is a valid hex digit
func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F')
}

// MustParseRevert is like ParseRevert but panics on error
// Useful for tests where you expect the parsing to succeed
func MustParseRevert(hexData string) ContractError {
	err, parseErr := ParseRevert(hexData)
	if parseErr != nil {
		panic(fmt.Sprintf("MustParseRevert failed: %v", parseErr))
	}
	return err
}

// GetSelector extracts just the error selector without decoding
func GetSelector(hexData string) (string, error) {
	hexData = strings.TrimSpace(hexData)
	if strings.HasPrefix(hexData, "0x") {
		hexData = hexData[2:]
	}

	if len(hexData) < 8 {
		return "", fmt.Errorf("data too short for selector")
	}

	return "0x" + hexData[:8], nil
}
