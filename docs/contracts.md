# Contract Management

This document describes how smart contract bindings are managed in the Piri project.

## Overview

Piri uses smart contracts from the [FilOzone/pdp](https://github.com/FilOzone/pdp) repository for PDP (Provable Data Possession) functionality. These contracts are included as a git submodule, and Go bindings are automatically generated using `abigen`.

## Project Structure

```
contracts/pdp/                          # Git submodule containing contract source
├── src/                                # Solidity contract source files
│   ├── PDPVerifier.sol                 # Main PDP verification contract
│   └── IPDPProvingSchedule.sol         # Proving schedule interface
├── out/                                # Foundry build output (ignored by git)
└── cache/                              # Foundry cache (ignored by git)

pkg/pdp/service/contract/
├── internal/                           # Generated Go bindings (committed)
│   ├── pdp_verifier.go                 # Generated from PDPVerifier.sol
│   └── pdp_proving_schedule.go         # Generated from IPDPProvingSchedule.sol
├── contract.go                         # Wrapper interfaces
├── addresses.go                        # Contract addresses (via config)
├── VERSION                             # Version tracking file
└── generate.go                         # go:generate hook

scripts/
└── generate-contracts.sh               # Contract generation script
```

## Version Tracking

The `VERSION` file tracks the exact state of contract generation:

```bash
PDP_CONTRACT_VERSION=08751740fe60ec0d45c7c021c26665670f44ed97  # Submodule commit SHA
PDP_CONTRACT_COMMIT=08751740fe60ec0d45c7c021c26665670f44ed97   # Same as above
GENERATED_AT=2025-09-06T12:02:59Z                              # Generation timestamp  
FORGE_METHOD=native                                            # Build method used
```

This enables:
- **Reproducible builds**: Anyone can regenerate identical bindings from the same commit
- **Audit trail**: Clear history of when bindings were last updated
- **Debugging**: Know exactly which contract version was used

## Development Workflow

### Prerequisites

Before working with contracts, ensure you have the required tools:

```bash
# Install Foundry (for contract compilation)
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Install Go development tools
make tools

# Verify tools are available
forge --version
abigen --version
```

### Initial Setup

For new developers or clean environments:

```bash
# Clone the repository
git clone <repository-url>
cd piri

# Initialize contract submodule
git submodule update --init

# Install development tools
make tools

# Generate initial contract bindings
make generate-contracts
```

## Contract Generation Methods

There are several ways to regenerate contract bindings:

### Method 1: Makefile (Recommended)

```bash
# Generate bindings from current submodule state
make generate-contracts

# Verify bindings are up-to-date (useful for CI)
make verify-contracts

# Update to latest contract version and regenerate
make contracts-update
```

### Method 2: Go Generate

```bash
# Generate contracts for specific package
go generate ./pkg/pdp/service/contract/

# Generate all contracts in project
go generate ./...
```

### Method 3: Direct Script Execution

```bash
# Run the generation script directly
./scripts/generate-contracts.sh
```

## Updating Contract Versions

### Automatic Update (Latest Version)

```bash
make contracts-update
```

This will:
1. Update the submodule to the latest remote version
2. Regenerate all bindings
3. Update the VERSION file  
4. Provide instructions for committing changes

### Manual Version Control

For more control over which version to use:

```bash
# Update submodule to specific version
cd contracts/pdp
git fetch
git checkout v1.2.0  # or specific commit hash
cd ../..

# Regenerate bindings
make generate-contracts

# Commit changes
git add contracts/pdp pkg/pdp/service/contract/
git commit -m "Update PDP contracts to v1.2.0"
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Verify Contracts
on: [push, pull_request]

jobs:
  verify-contracts:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install Foundry
        uses: foundry-rs/foundry-toolchain@v1
      
      - name: Install tools
        run: make tools
      
      - name: Verify contract bindings are up-to-date
        run: make verify-contracts
```

### Local Pre-commit Hook

```bash
#!/bin/sh
# .git/hooks/pre-commit
make verify-contracts
```

## Build Methods

The generation script automatically detects the best available method:

### Native Foundry (Preferred)
- Uses locally installed `forge` command
- Faster execution
- Better error messages

### Docker Fallback
- Uses `ghcr.io/foundry-rs/foundry:latest` image
- Works when Foundry is not locally installed
- Requires Docker to be available

The `FORGE_METHOD` field in the VERSION file indicates which method was used.

## Troubleshooting

### Common Issues

#### "forge not found" or "Docker not available"

**Solution**: Install Foundry or Docker:

```bash
# Option 1: Install Foundry (recommended)
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Option 2: Install Docker
# Follow Docker installation guide for your platform
```

#### "abigen not found"

**Solution**: Install Go tools:

```bash
make tools

# If PATH issue persists:
export PATH=$PATH:$(go env GOPATH)/bin
```

#### "jq not found"

**Solution**: Install jq for JSON processing:

```bash
# macOS
brew install jq

# Ubuntu/Debian  
sudo apt-get install jq

# CentOS/RHEL
sudo yum install jq
```

#### Generated files don't compile

**Cause**: The wrapper interfaces in `contract.go` may be out of sync with newly generated bindings.

**Solution**: Update the interface definitions to match the generated code. Common changes include:
- Method names (e.g., `FindRootIds` → `FindPieceIds`)
- Type names (e.g., `PDPVerifierRootIdAndOffset` → `IPDPTypesPieceIdAndOffset`)
- Method signatures

#### Submodule issues

```bash
# Reset submodule to clean state
git submodule deinit contracts/pdp
git submodule update --init contracts/pdp

# Update submodule to latest
git submodule update --remote contracts/pdp
```

#### Permission denied on script execution

```bash
chmod +x scripts/generate-contracts.sh
```

### Debug Mode

For detailed output during generation:

```bash
# Run script with verbose output
bash -x ./scripts/generate-contracts.sh
```

### Clean Build

To start completely fresh:

```bash
# Remove build artifacts
rm -rf contracts/pdp/out contracts/pdp/cache

# Regenerate everything
make generate-contracts
```

## Advanced Usage

### Custom Contract Versions

To pin to a specific development branch:

```bash
cd contracts/pdp  
git checkout feature/new-proving-system
cd ../..
make generate-contracts
```

### Local Contract Development

If you're developing contracts locally:

```bash
# Point submodule to local development version
cd contracts/pdp
git remote add local /path/to/local/pdp/repo
git fetch local
git checkout local/my-feature-branch
cd ../..
make generate-contracts
```

### Multiple Contract Versions

For testing against different contract versions:

```bash
# Save current state
git stash

# Test against different version
cd contracts/pdp && git checkout v1.0.0 && cd ../..
make generate-contracts
go test ./...

# Test against another version  
cd contracts/pdp && git checkout v2.0.0 && cd ../..
make generate-contracts
go test ./...

# Restore original state
git stash pop
```

## Best Practices

1. **Always verify after changes**: Run `make verify-contracts` before committing
2. **Use specific versions**: Pin to tagged releases rather than branch tips in production
3. **Test after updates**: Run the full test suite after updating contracts
4. **Document breaking changes**: Note interface changes in commit messages
5. **Coordinate updates**: Ensure all team members update submodules together
6. **Review generated code**: Check diffs in generated files for unexpected changes

## Security Considerations

- **Verify contract sources**: Always review contract changes before updating
- **Check commit signatures**: Ensure contract updates come from trusted sources  
- **Test thoroughly**: Contract changes can have significant security implications
- **Audit trails**: The VERSION file provides accountability for contract updates

## Support

If you encounter issues not covered in this guide:

1. Check the [FilOzone/pdp repository](https://github.com/FilOzone/pdp) for contract-specific issues
2. Review recent changes in the contract repository that might affect bindings
3. Ensure your development environment matches the documented prerequisites
4. Consider updating to the latest version of Foundry and Go tools
