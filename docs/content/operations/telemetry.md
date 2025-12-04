# Telemetry and Analytics

Piri collects telemetry data to help developers understand how the software is being used and improve the experience of running Piri nodes. 
This includes some pseudonymous identifiers (DIDs and Ethereum addresses) that are part of your node's public identity on the network. 
Telemetry can be disabled at any time.

## Why We Collect Telemetry

Telemetry helps the Piri development team to:

- Understand which versions of Piri are actively being used in the network
- Monitor the health and reliability of the network
- Identify common deployment patterns and configurations
- Prioritize development efforts based on actual usage
- Debug issues more effectively by understanding the runtime environment

## What Data Is Collected

Piri collects information about your node, including pseudonymous identifiers that are part of your node's public network identity:

### Base Server Information (All Node Types)

- **Version**: The Piri software version you're running
- **Commit**: The git commit hash of your build
- **Built By**: The builder information (typically automated build system)
- **Build Date**: When your version was compiled
- **Start Time**: Unix timestamp of when the server started
- **Server Type**: The type of server (e.g., "pdp" or "ucan")

### PDP Server Specific Data

- **Ethereum Address**: Your node's Ethereum address for PDP operations

### UCAN Server Specific Data

- **DID**: Your server's Decentralized Identifier
- **Indexing Service DID**: The DID of the indexing service you're connected to
- **Indexing Service URL**: The URL of the indexing service (public endpoint)
- **Upload Service DID**: The DID of the upload service you're connected to
- **Upload Service URL**: The URL of the upload service (public endpoint)

## What Data Is NOT Collected

We do NOT collect:

- Private keys or any sensitive cryptographic material
- Personal information beyond what's listed above
- IP addresses or location data

## How to Opt Out

If you prefer not to share telemetry data, you can disable it by setting the `PIRI_DISABLE_ANALYTICS` environment variable:

```bash
export PIRI_DISABLE_ANALYTICS=1
```

To make this permanent, add the export to your shell configuration file (e.g., `.bashrc`, `.zshrc`).

## Data Retention and Usage

- Telemetry data is used solely for improving Piri
- Data is retained for a reasonable period to analyze trends
- We do not sell or share this data with third parties
- All data handling follows industry best practices for privacy and security

## Technical Implementation

Telemetry is implemented using OpenTelemetry, an industry-standard observability framework. The telemetry system:

- Initializes during startup unless `PIRI_DISABLE_ANALYTICS` is set
- Records server information once at startup
- Has a 10-second timeout for initialization to prevent blocking
- Logs warnings if telemetry fails but continues normal operation

## Questions or Concerns

If you have questions about telemetry or privacy, please:

- Open an issue on our [GitHub repository](https://github.com/storacha/piri)
- Contact the development team through official channels

We're committed to transparency and will continue to document any changes to telemetry collection.