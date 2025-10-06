# Downloading Piri

This section covers downloading the Piri binary to your local system.

## Prerequisites

Before downloading, ensure you have:
- ✅ [Completed system prerequisites](./prerequisites.md)

## Download Pre-compiled Binary

Download the latest release from [v0.0.14](https://github.com/storacha/piri/releases/tag/v0.0.14):

### For Linux AMD64
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.14/piri_0.0.14_linux_amd64.tar.gz
tar -xzf piri_0.0.14_linux_amd64.tar.gz
```

### For Linux ARM64
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.14/piri_0.0.14_linux_arm64.tar.gz
tar -xzf piri_0.0.14_linux_arm64.tar.gz
```

## Verify Download

```bash
# Verify the binary works
./piri version
```

The output version should match the version you downloaded (v0.0.14).

## Important Note

**Do not move the binary to `/usr/local/bin` or add it to your system PATH yet.**

After you complete the initialization step, you'll choose an installation method:
- **Service Installation (Recommended)**: Automatically manages the binary location and provides auto-updates
- **Manual Installation**: You'll move the binary to your preferred location

Keeping the binary in your current directory for now prevents conflicts and ensures a smooth installation experience.

---

## Next Steps

After downloading Piri:
- [Generate Cryptographic Keys](./key-generation.md)
- [Configure TLS Termination](./tls-termination.md)
