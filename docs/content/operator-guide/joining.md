# Joining the Network

Becoming a Forge operator involves an approval process followed by on-chain registration.

## Getting Approved

Operator approval is handled by Storacha. Before you can join the network, you'll need to:

1. **Contact Storacha** to express interest in becoming an operator
2. **Demonstrate capability** — Storacha evaluates prospective operators for reliable infrastructure, adequate resources, and operational readiness
3. **Receive approval** — Once approved, you'll be authorized to register on-chain

This vetting process ensures the network maintains quality and reliability for users who depend on it.

## After Approval

Once approved, you complete these steps to join the network:

### 1. Register with the Provider Registry

Your operator address must be registered in the Provider Registry smart contract. This establishes your identity on-chain as an authorized storage provider.

### 2. Pay the Registration Fee

Registration requires a **5 FIL** fee. This fee:

- Commits you as a serious participant in the network
- Is paid to the smart contract during registration
- Is a one-time cost per operator

### 3. Create Your Dataset

A **dataset** (also called a proof set) is your on-chain commitment to store data. It:

- Has a unique ID assigned by the smart contract
- Tracks the data roots you've committed to prove
- Defines your proving schedule (challenge windows, proving periods)

The `piri init` command handles dataset creation as part of node initialization, but the concept is important: your dataset is what ties your stored data to your on-chain obligations.

## Setting Up Your Node

With approval in hand, you're ready to set up infrastructure. The detailed setup guides cover each step:

| Step                                         | Guide                                          |
|----------------------------------------------|------------------------------------------------|
| Hardware, OS, network requirements           | [Prerequisites](../setup/prerequisites.md)     |
| Downloading and verifying the binary         | [Installation](../setup/installation.md)       |
| Creating your node identity (DID) and wallet | [Key Generation](../setup/key-generation.md)   |
| Configuring HTTPS with a reverse proxy       | [TLS Termination](../setup/tls-termination.md) |
| Initializing and running your node           | [Piri Server](../setup/piri-server.md)         |

Before starting setup, ensure you have a Lotus node ready. See [Lotus Node Setup](lotus.md) for requirements.

## Network Selection

Forge operates on two networks:

| Network        | Chain                | Use Case                                        |
|----------------|----------------------|-------------------------------------------------|
| `forge-prod`   | Filecoin Mainnet     | Production — real data, real payments           |
| `warm-staging` | Filecoin Calibration | Testing — validate your setup before production |

Start on `warm-staging` to verify everything works, then move to `forge-prod` for production operation.

See [Concepts > Networks](../concepts/networks.md) for service endpoints and contract addresses.
