package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/crypto/sha3"
)

// ErrorABI represents an error definition from the ABI JSON
type ErrorABI struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Inputs []struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		InternalType string `json:"internalType"`
	} `json:"inputs"`
}

// ContractABI represents the full contract ABI
type ContractABI struct {
	ABI []ErrorABI `json:"abi"`
}

// ErrorInfo contains processed information about an error
type ErrorInfo struct {
	Name      string
	Selector  string
	Signature string
	Inputs    []ErrorInput
	HasInputs bool
}

// ErrorInput represents a single error parameter
type ErrorInput struct {
	Name      string
	SolType   string
	GoType    string
	IsEnum    bool
	IsString  bool
	IsAddress bool
	IsUint    bool
	IsBytes   bool
}

func main() {
	abiPath := flag.String("abi", "", "Path to the contract ABI JSON file")
	outDir := flag.String("out", "", "Output directory for generated Go files")
	flag.Parse()

	if *abiPath == "" || *outDir == "" {
		fmt.Println("Usage: error-binding-generator -abi <path> -out <dir>")
		os.Exit(1)
	}

	// Read ABI file
	data, err := os.ReadFile(*abiPath)
	if err != nil {
		fmt.Printf("Error reading ABI file: %v\n", err)
		os.Exit(1)
	}

	// Parse ABI JSON
	var contractABI ContractABI
	if err := json.Unmarshal(data, &contractABI); err != nil {
		fmt.Printf("Error parsing ABI JSON: %v\n", err)
		os.Exit(1)
	}

	// Filter and process errors
	var errors []ErrorInfo
	for _, item := range contractABI.ABI {
		if item.Type != "error" {
			continue
		}

		errorInfo := processError(item)
		errors = append(errors, errorInfo)
	}

	// Sort errors by name for consistent output
	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Name < errors[j].Name
	})

	fmt.Printf("Processed %d errors\n", len(errors))

	// Create output directory
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate files
	if err := generateErrorsFile(*outDir, errors); err != nil {
		fmt.Printf("Error generating errors.go: %v\n", err)
		os.Exit(1)
	}

	if err := generateDecodersFile(*outDir, errors); err != nil {
		fmt.Printf("Error generating decoders.go: %v\n", err)
		os.Exit(1)
	}

	if err := generateHelpersFile(*outDir, errors); err != nil {
		fmt.Printf("Error generating helpers.go: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully generated Go bindings!")
}

// processError extracts information from an error ABI definition
func processError(errorABI ErrorABI) ErrorInfo {
	info := ErrorInfo{
		Name:      errorABI.Name,
		Signature: buildSignature(errorABI),
		HasInputs: len(errorABI.Inputs) > 0,
	}

	info.Selector = computeSelector(info.Signature)

	for _, input := range errorABI.Inputs {
		info.Inputs = append(info.Inputs, ErrorInput{
			Name:      capitalize(input.Name),
			SolType:   input.Type,
			GoType:    solTypeToGoType(input.Type, input.InternalType),
			IsEnum:    strings.Contains(input.InternalType, "enum"),
			IsString:  input.Type == "string",
			IsAddress: input.Type == "address",
			IsUint:    strings.HasPrefix(input.Type, "uint"),
			IsBytes:   strings.HasPrefix(input.Type, "bytes"),
		})
	}

	return info
}

// buildSignature creates the canonical error signature
func buildSignature(errorABI ErrorABI) string {
	if len(errorABI.Inputs) == 0 {
		return errorABI.Name + "()"
	}

	types := make([]string, len(errorABI.Inputs))
	for i, input := range errorABI.Inputs {
		types[i] = input.Type
	}

	return fmt.Sprintf("%s(%s)", errorABI.Name, strings.Join(types, ","))
}

// computeSelector computes the 4-byte error selector
func computeSelector(signature string) string {
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(signature))
	return "0x" + hex.EncodeToString(hash.Sum(nil)[:4])
}

// solTypeToGoType converts Solidity types to Go types
func solTypeToGoType(solType, internalType string) string {
	// Handle enums (encoded as uint8 in ABI but we'll use uint8 in Go)
	if strings.Contains(internalType, "enum") {
		return "uint8"
	}

	switch solType {
	case "address":
		return "common.Address"
	case "uint8":
		return "uint8"
	case "uint16":
		return "uint16"
	case "uint32":
		return "uint32"
	case "uint64":
		return "uint64"
	case "uint256":
		return "*big.Int"
	case "string":
		return "string"
	case "bytes":
		return "[]byte"
	case "bool":
		return "bool"
	default:
		if strings.HasPrefix(solType, "uint") {
			return "*big.Int"
		}
		if strings.HasPrefix(solType, "bytes") {
			return "[]byte"
		}
		return "interface{}"
	}
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// generateErrorsFile generates the error type definitions
func generateErrorsFile(outDir string, errors []ErrorInfo) error {
	tmpl := template.Must(template.New("errors").Parse(errorsTemplate))

	f, err := os.Create(filepath.Join(outDir, "errors.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, struct {
		Errors []ErrorInfo
	}{
		Errors: errors,
	})
}

// generateDecodersFile generates decoder functions and the selector map
func generateDecodersFile(outDir string, errors []ErrorInfo) error {
	tmpl := template.Must(template.New("decoders").Parse(decodersTemplate))

	f, err := os.Create(filepath.Join(outDir, "decoders.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, struct {
		Errors []ErrorInfo
	}{
		Errors: errors,
	})
}

// generateHelpersFile generates helper functions like IsXxx()
func generateHelpersFile(outDir string, errors []ErrorInfo) error {
	tmpl := template.Must(template.New("helpers").Parse(helpersTemplate))

	f, err := os.Create(filepath.Join(outDir, "helpers.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, struct {
		Errors []ErrorInfo
	}{
		Errors: errors,
	})
}

const errorsTemplate = `// Code generated by error-binding-generator. DO NOT EDIT.

package evmerrors

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// ContractError is the base interface for all contract errors
type ContractError interface {
	error
	ErrorName() string
	ErrorSelector() string
}

{{range .Errors}}
// {{.Name}} represents the {{.Name}} error
type {{.Name}} struct {
{{- range .Inputs}}
	{{.Name}} {{.GoType}}
{{- end}}
}

func (e *{{.Name}}) Error() string {
{{- if .HasInputs}}
	return fmt.Sprintf("{{.Name}}({{range $i, $input := .Inputs}}{{if $i}}, {{end}}{{if $input.IsString}}{{$input.Name}}=%s{{else if $input.IsAddress}}{{$input.Name}}=%s{{else}}{{$input.Name}}=%v{{end}}{{end}})", {{range $i, $input := .Inputs}}{{if $i}}, {{end}}{{if $input.IsAddress}}e.{{$input.Name}}.Hex(){{else}}e.{{$input.Name}}{{end}}{{end}})
{{- else}}
	return "{{.Name}}()"
{{- end}}
}

func (e *{{.Name}}) ErrorName() string {
	return "{{.Name}}"
}

func (e *{{.Name}}) ErrorSelector() string {
	return "{{.Selector}}"
}

{{end}}
`

const decodersTemplate = `// Code generated by error-binding-generator. DO NOT EDIT.

package evmerrors

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// DecoderFunc is a function that decodes error parameters from hex data
type DecoderFunc func(data []byte) (ContractError, error)

// ErrorDecoders maps error selectors to their decoder functions
var ErrorDecoders = map[string]DecoderFunc{
{{- range .Errors}}
	"{{.Selector}}": decode{{.Name}}, // {{.Signature}}
{{- end}}
}

{{range .Errors}}
// decode{{.Name}} decodes {{.Name}} error parameters
func decode{{.Name}}(data []byte) (ContractError, error) {
{{- if not .HasInputs}}
	return &{{.Name}}{}, nil
{{- else}}
	// Define ABI arguments
	arguments := abi.Arguments{
{{- range $i, $input := .Inputs}}
		{Type: mustNewType("{{$input.SolType}}")},
{{- end}}
	}

	// Unpack the data
	values, err := arguments.Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack error data: %w", err)
	}

	if len(values) != {{len .Inputs}} {
		return nil, fmt.Errorf("expected {{len .Inputs}} values, got %d", len(values))
	}

	return &{{.Name}}{
{{- range $i, $input := .Inputs}}
		{{$input.Name}}: {{if $input.IsAddress}}values[{{$i}}].(common.Address){{else if $input.IsString}}values[{{$i}}].(string){{else if eq $input.GoType "uint8"}}uint8(values[{{$i}}].(uint8)){{else if eq $input.GoType "*big.Int"}}values[{{$i}}].(*big.Int){{else}}values[{{$i}}].({{$input.GoType}}){{end}},
{{- end}}
	}, nil
{{- end}}
}

{{end}}

// mustNewType creates a new ABI type, panicking on error
func mustNewType(t string) abi.Type {
	typ, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create ABI type %s: %v", t, err))
	}
	return typ
}
`

const helpersTemplate = `// Code generated by error-binding-generator. DO NOT EDIT.

package evmerrors

{{range .Errors}}
// Is{{.Name}} checks if the error is a {{.Name}} error
func Is{{.Name}}(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*{{.Name}})
	return ok
}

{{end}}

// GetErrorName returns the error name if it's a contract error, otherwise returns "UnknownError"
func GetErrorName(err error) string {
	if cerr, ok := err.(ContractError); ok {
		return cerr.ErrorName()
	}
	return "UnknownError"
}

// GetErrorSelector returns the error selector if it's a contract error, otherwise returns empty string
func GetErrorSelector(err error) string {
	if cerr, ok := err.(ContractError); ok {
		return cerr.ErrorSelector()
	}
	return ""
}
`
