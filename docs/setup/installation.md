# Installing Piri

This section covers the installation of the Piri binary.

## Prerequisites

Before installing, ensure you have:
- ✅ [Completed system prerequisites](./prerequisites.md)

## Download Pre-compiled Binary

Download the latest release from [v0.0.14](https://github.com/storacha/piri/releases/tag/v0.0.14):

### For Linux AMD64
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.14/piri_0.0.14_linux_amd64.tar.gz
tar -xzf piri_0.0.14_linux_amd64.tar.gz
sudo mv piri /usr/local/bin/piri
```

### For Linux ARM64
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.14/piri_0.0.14_linux_arm64.tar.gz
tar -xzf piri_0.0.14_linux_arm64.tar.gz
sudo mv piri /usr/local/bin/piri
```

## Verify Installation

```bash
# The output version should match the version installed via the above command
piri version
```

---

## Next Steps

After installing Piri:
- [Generate Cryptographic Keys](./key-generation.md)
- [Configure TLS Termination](./tls-termination.md)