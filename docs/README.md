# Piri Documentation

Welcome to the Piri documentation!
Piri is the software run by all storage providers on the Storacha network. 
Piri can run entirely on its own with no software other than Filecoin Lotus, or it can integrate with a Filecoin storage provider operation running Curio.

## 🚀 Quick Start

New to Piri? Start here:

1. **[Getting Started Guide](./getting-started.md)** - Decision tree to help you choose the right setup
2. **[Architecture Overview](./architecture.md)** - Understand how Piri components work together
3. **[Prerequisites](./common/prerequisites.md)** - Ensure your system is ready

## 📚 Documentation Structure

### Common Procedures

Reusable guides referenced throughout the documentation:

- **[Prerequisites](./common/prerequisites.md)** - System requirements and dependencies
- **[Piri Installation](./common/piri-installation.md)** - Installing the Piri binary
- **[PEM File Generation](./common/pem-file-generation.md)** - Creating and managing Ed25519 keys
- **[TLS Termination](./common/tls-termination.md)** - Setting up HTTPS with Nginx or alternatives

### Setup Guides

Step-by-step guides for different components:

- **[PDP Server Setup](./guides/pdp-server-piri.md)** - Run a Piri PDP server for data storage and proofs
- **[UCAN Server Setup](./guides/ucan-server.md)** - Run a Piri UCAN server for client uploads

### Integration Guides

Advanced deployment scenarios:

- **[Piri with Curio](./integrations/piri-with-curio.md)** - Use Curio as your PDP backend
- **[Full Stack Setup](./integrations/full-stack-setup.md)** - Deploy both UCAN and PDP servers together

## 🎯 Which Guide Should I Follow?

### "I want to run a complete Piri storage provider"
→ Follow the **[Full Stack Setup](./integrations/full-stack-setup.md)** guide

### "I already run Curio and want to add Storacha support"
→ Follow the **[Piri with Curio](./integrations/piri-with-curio.md)** guide

### "I just want to understand the architecture"
→ Read the **[Architecture Overview](./architecture.md)**

### "I need to run only a PDP server"
→ Follow the **[PDP Server Setup](./guides/pdp-server-piri.md)** guide

### "I need to run only a UCAN server"
→ Follow the **[UCAN Server Setup](./guides/ucan-server.md)** guide

## 🔧 Common Tasks

### Generate a PEM File
```bash
piri id gen -t=pem
```
See the [PEM File Generation](./common/pem-file-generation.md) guide for details.

### Check Server Health
```bash
# UCAN server
curl https://your-domain.com/health

# PDP server
curl https://pdp.your-domain.com/health
```

### Create a Proof Set
```bash
piri proofset create --key-file=service.pem --pdp-server-url=https://pdp-domain.com
```

## 🆘 Getting Help

### Troubleshooting

Each guide includes a troubleshooting section. Common issues:

- **Connection errors**: Check [TLS setup](./common/tls-termination.md) and firewall rules
- **Authentication failures**: Verify [PEM file](./common/pem-file-generation.md) and delegations
- **Installation issues**: Review [prerequisites](./common/prerequisites.md) and [installation](./common/piri-installation.md)

### Support Channels

- **GitHub Issues**: [github.com/storacha/piri/issues](https://github.com/storacha/piri/issues)
- **Documentation Issues**: [github.com/storacha/piri/issues](https://github.com/storacha/piri/issues)
- **Community**: Join the Storacha community channels

## 📋 Documentation Index

### Concepts
- [Architecture Overview](./architecture.md)
- [Getting Started](./getting-started.md)

### Common Procedures
- [Prerequisites](./common/prerequisites.md)
- [Piri Installation](./common/piri-installation.md)
- [PEM File Generation](./common/pem-file-generation.md)
- [TLS Termination](./common/tls-termination.md)

### Setup Guides
- [PDP Server Setup](./guides/pdp-server-piri.md)
- [UCAN Server Setup](./guides/ucan-server.md)

### Integrations
- [Piri with Curio](./integrations/piri-with-curio.md)
- [Full Stack Setup](./integrations/full-stack-setup.md)

### Legacy Guides
- [Original PDP Server Guide](./PIRI-PDP-SERVER-GUIDE.md) (deprecated)
- [Original UCAN Server Guide](./PIRI-UCAN-SERVER-GUIDE.md) (deprecated)

## 🔄 Version Information

This documentation is for Piri version 1.x. For the latest updates:

```bash
# Check your version
piri version

# Update Piri
cd piri && git pull && make calibnet
```

## 📝 Contributing

Help improve these docs! If you find issues or have suggestions:

1. Open an issue describing the improvement
2. Submit a pull request with your changes
3. Follow the existing documentation style

Remember: Clear documentation helps everyone succeed with Piri!