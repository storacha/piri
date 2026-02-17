# Networks

Piri operates on Storacha networks, which define the services and smart contracts it communicates with.

## forge-prod

Production network on Filecoin Mainnet.

**Chain ID:** 314

### Services

| Service        | URL                                        | DID                                       |
|----------------|--------------------------------------------|-------------------------------------------|
| Upload         | `https://up.forge.storacha.network`        | `did:web:up.forge.storacha.network`       |
| Indexing       | `https://indexer.forge.storacha.network`   | `did:web:indexer.forge.storacha.network`  |
| Egress Tracker | `https://etracker.forge.storacha.network`  | `did:web:etracker.forge.storacha.network` |
| Signing        | `https://signer.forge.storacha.network`    | `did:web:signer.forge.storacha.network`   |
| Registrar      | `https://registrar.forge.storacha.network` | -                                         |
| IPNI           | `https://ipni.forge.storacha.network`      | -                                         |

### Smart Contracts

| Contract          | Address                                                                                                                            |
|-------------------|------------------------------------------------------------------------------------------------------------------------------------|
| PDP Verifier      | [`0xBADd0B92C1c71d02E7d520f64c0876538fa2557F`](https://filecoin.blockscout.com/address/0xBADd0B92C1c71d02E7d520f64c0876538fa2557F) |
| Provider Registry | [`0xf55dDbf63F1b55c3F1D4FA7e339a68AB7b64A5eB`](https://filecoin.blockscout.com/address/0xf55dDbf63F1b55c3F1D4FA7e339a68AB7b64A5eB) |
| PDP Service       | [`0x56e53c5e7F27504b810494cc3b88b2aa0645a839`](https://filecoin.blockscout.com/address/0x56e53c5e7F27504b810494cc3b88b2aa0645a839) |
| PDP Service View  | [`0x778Bbb8F50d759e2AA03ab6FAEF830Ca329AFF9D`](https://filecoin.blockscout.com/address/0x778Bbb8F50d759e2AA03ab6FAEF830Ca329AFF9D) |
| Payments          | [`0x23b1e018F08BB982348b15a86ee926eEBf7F4DAa`](https://filecoin.blockscout.com/address/0x23b1e018F08BB982348b15a86ee926eEBf7F4DAa) |
| USDFC Token       | [`0x80B98d3aa09ffff255c3ba4A241111Ff1262F045`](https://filecoin.blockscout.com/address/0x80B98d3aa09ffff255c3ba4A241111Ff1262F045) |

**Payer Address (Storacha) :** [`0x3c1ae7a70a2b51458fcb7927fd77aae408a1b857`](https://filecoin.blockscout.com/address/0x3c1ae7a70a2b51458fcb7927fd77aae408a1b857)

## warm-staging

Staging network on Filecoin Calibration testnet. Use this for testing before deploying to production.

**Chain ID:** 314159

### Services

| Service        | URL                                               | DID                                              |
|----------------|---------------------------------------------------|--------------------------------------------------|
| Upload         | `https://staging.up.warm.storacha.network`        | `did:web:staging.up.warm.storacha.network`       |
| Indexing       | `https://staging.indexer.warm.storacha.network`   | `did:web:staging.indexer.warm.storacha.network`  |
| Egress Tracker | `https://staging.etracker.warm.storacha.network`  | `did:web:staging.etracker.warm.storacha.network` |
| Signing        | `https://staging.signer.warm.storacha.network`    | `did:web:staging.signer.warm.storacha.network`   |
| Registrar      | `https://staging.registrar.warm.storacha.network` | -                                                |
| IPNI           | `https://staging.ipni.warm.storacha.network`      | -                                                |

### Smart Contracts

| Contract          | Address                                                                                                                                    |
|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| PDP Verifier      | [`0x85e366Cf9DD2c0aE37E963d9556F5f4718d6417C`](https://filecoin-testnet.blockscout.com/address/0x85e366Cf9DD2c0aE37E963d9556F5f4718d6417C) |
| Provider Registry | [`0x6A96aaB210B75ee733f0A291B5D8d4A053643979`](https://filecoin-testnet.blockscout.com/address/0x6A96aaB210B75ee733f0A291B5D8d4A053643979) |
| PDP Service       | [`0x0c6875983B20901a7C3c86871f43FdEE77946424`](https://filecoin-testnet.blockscout.com/address/0x0c6875983B20901a7C3c86871f43FdEE77946424) |
| PDP Service View  | [`0xEAD67d775f36D1d2894854D20e042C77A3CC20a5`](https://filecoin-testnet.blockscout.com/address/0xEAD67d775f36D1d2894854D20e042C77A3CC20a5) |
| Payments          | [`0x09a0fDc2723fAd1A7b8e3e00eE5DF73841df55a0`](https://filecoin-testnet.blockscout.com/address/0x09a0fDc2723fAd1A7b8e3e00eE5DF73841df55a0) |
| USDFC Token       | [`0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0`](https://filecoin-testnet.blockscout.com/address/0xb3042734b608a1B16e9e86B374A3f3e389B4cDf0) |

**Payer Address (Storacha):** [`0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD`](https://filecoin-testnet.blockscout.com/address/0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD)

