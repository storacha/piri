# Getting Started with Piri

Choose your path:

## 🚀 Running a Storage Provider

### Option 1: Full Piri Stack (Recommended for New Providers)
Run both UCAN and PDP servers for complete storage provider functionality.

→ 📖 **[Full Stack Setup Guide](./integrations/full-stack-setup.md)**

### Option 2: Piri with Curio (Recommended for Existing Operators already using Curio)
Already running Curio? Add just the UCAN server to join Storacha network.

→ 📖 **[Piri with Curio Integration](./integrations/piri-with-curio.md)**

## 👩‍💻 Contributing to Piri

Want to contribute? Check out:
- [Architecture Overview](./architecture.md) - Understand the system
- Set up local development using the [Full Stack Guide](./integrations/full-stack-setup.md)
- [GitHub Issues](https://github.com/storacha/piri/issues) - Find tasks to work on

## Prerequisites

Before starting, ensure you have:
- Go 1.24+, Git, Make, jq
- See [detailed prerequisites](./common/prerequisites.md)

## Quick Start

### Download Pre-compiled Binary (Recommended)

```bash
# For Linux AMD64
wget https://github.com/storacha/piri/releases/download/v0.0.6/piri_0.0.6_linux_amd64.tar.gz
tar -xzf piri_0.0.6_linux_amd64.tar.gz
sudo mv piri /usr/local/bin/

# For Linux ARM64
wget https://github.com/storacha/piri/releases/download/v0.0.6/piri_0.0.6_linux_arm64.tar.gz
tar -xzf piri_0.0.6_linux_arm64.tar.gz
sudo mv piri /usr/local/bin/

# Generate identity
piri id gen -t=pem > service.pem
```

### Build from Source (Alternative)

```bash
# Clone and build
git clone https://github.com/storacha/piri
cd piri
git checkout v0.0.6
make

# Generate identity
piri id gen -t=pem > service.pem
```

Then follow your chosen guide above.
