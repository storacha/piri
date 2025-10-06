# Getting Started with Piri

This guide shows you how to set up a Piri node from the beginning. Follow each step in order for the best results.

## Overview

To set up Piri, you need to:
1. Prepare your system and network
2. Download Piri
3. Create your keys and wallet
4. Initialize your node configuration
5. Choose and complete your installation method
6. Validate your setup

## Complete Setup Guide

### Step 1: [Prerequisites](./setup/prerequisites.md)
Set up your system, network, and Filecoin node

### Step 2: [Download Piri](./setup/download.md)
Download the Piri binary to your local system

### Step 3: [Generate Keys](./setup/key-generation.md)
Create your identity key and wallet

### Step 4: [Configure TLS](./setup/tls-termination.md)
Set up secure connections (HTTPS) for your domain

### Step 5: [Initialize Configuration](./setup/initialization.md)
Run `piri init` to create your configuration file

### Step 6: [Choose Installation Method](./setup/choosing-installation.md)
**Important decision point:** Choose how to install and run Piri

- **[Service Installation](./setup/service-installation.md)** (Recommended for production)
  - Automatic updates and reliability
  - Best for production storage providers

- **[Manual Installation](./setup/manual-installation.md)** (For development/testing)
  - Full control over updates
  - Best for temporary setups

### Step 7: [Validate](./setup/validation.md)
Test that everything works

## After Installation

### For Service Installations

- **[Service Management](./setup/service-management.md)** - Managing your service installation

### For Manual Installations

- **[Updating Piri](./setup/updating.md)** - Manually update your installation

## Additional Resources

### [Configuration Reference](./setup/configuration.md)
Learn about Piri configuration options

### [Telemetry and Analytics](./telemetry.md)
Learn what data Piri collects and how to turn it off

---

After following this guide, you will have Piri running and ready to work with the Storacha network.