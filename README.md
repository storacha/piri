<div align="center">
  <img src="https://w3s.link/ipfs/bafybeidgd53ksarusewxkmf54ojnrmhneamtcvpqa7n7mi73k6hc7qlwym/centipede.png" alt="Storacha piri node logo" width="180" />
  <h1>Piri</h1>
  <p>A storage node that runs on the Storacha network.</p>
</div>

## What is Piri?

What's Piri? It's the _**P**rovable **I**nformation **R**etention **I**nterface_ - a Go-based storage node that's part of the Storacha network backbone. It works alongside other services like the [indexing service](https://github.com/storacha/indexing-service) and [upload service](https://github.com/storacha/upload-service) to enable decentralized storage with cryptographic proofs.

## Documentation

Get started with Piri by exploring our comprehensive documentation:

- **[🚀 Getting Started](./docs/getting-started.md)** - Complete setup guide to deploy Piri
- **[🏗️ Architecture](./docs/architecture.md)** - Understand how Piri works

### Setup Guides

Follow these guides in order to set up Piri:

1. **[Prerequisites](./docs/setup/prerequisites.md)** - System, network, and Filecoin requirements
2. **[Installation](./docs/setup/installation.md)** - Download and install Piri
3. **[Key Generation](./docs/setup/key-generation.md)** - Create your cryptographic identity
4. **[TLS Configuration](./docs/setup/tls-termination.md)** - Set up HTTPS for your domains
5. **[PDP Server Setup](./docs/guides/pdp-server.md)** - Deploy the storage backend
6. **[UCAN Server Setup](./docs/guides/ucan-server.md)** - Deploy the client-facing API
7. **[Validation](./docs/setup/validation.md)** - Test your deployment

> **Note:** Using Curio? See [Filecoin's PDP documentation](https://docs.filecoin.io/storage-providers/pdp/enable-pdp) for setup instructions. The Piri UCAN server can connect to Curio as an alternative to the Piri PDP server.

### Quick Links

- **New to Piri?** Start with the [Getting Started Guide](./docs/getting-started.md)
- **Want to understand the system?** Read the [Architecture Overview](./docs/architecture.md)

## Development

### Prerequisites for Development

To develop Piri, you'll need:

- Go 1.19 or later
- Foundry (for contract compilation)

### Setup Development Tools

Install required development tools:

```bash
# Install abigen and other Go tools
make tools

# If abigen is not in your PATH, add Go's bin directory:
export PATH=$PATH:$(go env GOPATH)/bin
```

### Contract Development

Piri uses smart contracts for PDP (Provable Data Possession). The contracts are managed via git submodules:

```bash
# Initialize contract submodules
git submodule update --init

# Generate contract bindings (after installing tools)
make generate-contracts
```

## Contributing

All welcome! Storacha is open-source. Please feel empowered to open a PR or an issue.

### Reporting Issues

Found a bug or have a feature request? Please [open an issue](https://github.com/storacha/piri/issues) on our GitHub repository.

## License

Dual-licensed under [Apache 2.0 OR MIT](LICENSE.md)
