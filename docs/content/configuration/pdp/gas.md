# Gas Fee Limits

Per-message-type gas fee limits with automatic deferral during high congestion.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `pdp.gas.max_fee.prove` | `0` (no limit) | `PIRI_PDP_GAS_MAX_FEE_PROVE` | Yes |
| `pdp.gas.max_fee.proving_period` | `0` (no limit) | `PIRI_PDP_GAS_MAX_FEE_PROVING_PERIOD` | Yes |
| `pdp.gas.max_fee.proving_init` | `0` (no limit) | `PIRI_PDP_GAS_MAX_FEE_PROVING_INIT` | Yes |
| `pdp.gas.max_fee.add_roots` | `0` (no limit) | `PIRI_PDP_GAS_MAX_FEE_ADD_ROOTS` | Yes |
| `pdp.gas.max_fee.default` | `0` (no limit) | `PIRI_PDP_GAS_MAX_FEE_DEFAULT` | Yes |
| `pdp.gas.retry_wait` | `5m` | `PIRI_PDP_GAS_RETRY_WAIT` | Yes |

## Overview

Piri sends several types of on-chain messages during normal operation. During network congestion, gas fees can spike dramatically — in one observed incident, base fees rose 489x above normal, costing a node operator ~3 FIL to onboard 0.5 TB of data.

Gas fee limits let you set a maximum cost (in wei) per message type. When the estimated gas cost exceeds the configured limit, Piri **defers the message** rather than sending it — the message is automatically retried after `retry_wait` elapses. During deferral:

- Data ingestion, storage, and retrieval continue normally
- Only on-chain message submission is paused
- Deferred messages do not consume the task's retry budget
- Messages are sent automatically once fees drop below the limit

**Dynamic Configuration:** All gas fee settings can be changed at runtime using the admin API. Changes take effect immediately on the next send attempt.

## Message Types

| Config Key | Message Type | Time Sensitivity |
|-----------|-------------|-----------------|
| `max_fee.prove` | Proof submission (`provePossession`) | High — must land within challenge window |
| `max_fee.proving_period` | Advancing proving period (`nextProvingPeriod`) | High — epoch-constrained |
| `max_fee.proving_init` | Initiating first proving period | High — per proof set |
| `max_fee.add_roots` | Adding roots to proof set | Low — data already stored |
| `max_fee.default` | Fallback for all other messages | Varies |

Messages without a dedicated config key (e.g., provider registration, proof set creation, root deletion) use the `default` limit.

## Fields

### `max_fee.prove`

Maximum gas fee (in wei) for proof submission messages. Set this high enough to ensure proofs land within their challenge window — missing a challenge window means the proof set is not proven for that period.

### `max_fee.proving_period`

Maximum gas fee (in wei) for advancing the proving period. Similar time sensitivity to `prove` — delays here postpone the next challenge cycle.

### `max_fee.proving_init`

Maximum gas fee (in wei) for initiating the first proving period on a proof set.

### `max_fee.add_roots`

Maximum gas fee (in wei) for adding roots to a proof set. This is the safest to cap aggressively — data is already stored and served, the root just isn't registered on-chain yet.

### `max_fee.default`

Fallback maximum gas fee (in wei) for any message type without a dedicated limit. Applies to one-time operations like provider registration, proof set creation, and root deletion.

### `retry_wait`

How long to wait before re-checking gas fees after a deferral. Default is 5 minutes. During sustained fee spikes, this prevents tight polling of the RPC endpoint.

## Recommendations

**Conservative (cost-sensitive):**

Set aggressive limits on non-time-sensitive operations and generous limits on proving:

```toml
[pdp.gas.max_fee]
prove = 100000000000000000          # 0.1 FIL
proving_period = 100000000000000000 # 0.1 FIL
proving_init = 50000000000000000    # 0.05 FIL
add_roots = 10000000000000000       # 0.01 FIL
default = 10000000000000000         # 0.01 FIL
```

**Permissive (availability-focused):**

Only cap the lowest-priority messages:

```toml
[pdp.gas.max_fee]
add_roots = 50000000000000000  # 0.05 FIL
default = 50000000000000000    # 0.05 FIL
```

**No limits (default):**

All values default to `0`, which means no gas fee checking — Piri pays whatever the network demands. This is the pre-existing behavior.

## Runtime Adjustment

During a gas spike, you can tighten limits without restarting:

```bash
# Check current settings
piri client admin config list

# Lower the limit for root additions
piri client admin config set pdp.gas.max_fee.add_roots 10000000000000000

# Increase the retry interval during sustained spikes
piri client admin config set pdp.gas.retry_wait 15m
```

To persist changes across restarts, add `--persist`:

```bash
piri client admin config set --persist pdp.gas.max_fee.add_roots 10000000000000000
```

## TOML

```toml
[pdp.gas]
retry_wait = "5m"

[pdp.gas.max_fee]
prove = 100000000000000000
proving_period = 100000000000000000
proving_init = 50000000000000000
add_roots = 10000000000000000
default = 10000000000000000
```
