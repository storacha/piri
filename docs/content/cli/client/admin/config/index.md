# config

Manage dynamic configuration values at runtime.

## How It Works

The config file is the source of truth. Runtime changes are ephemeral by default—they override behavior without touching the file. A reload restores the file's authority. The `--persist` flag exists for operators who mean it: change the behavior and update the file so the two agree.

This prevents the most common configuration surprise: a system whose running state has silently diverged from its config file, discovered during a restart nobody planned for.

**Example workflow:**

A Piri node starts with this config file:

```toml
[pdp.aggregation.manager]
poll_interval = "30s"
```

The node uses a poll interval of 30 seconds.

**Runtime override (ephemeral):**

```
piri client admin config set pdp.aggregation.manager.poll_interval 5m
```

The node now uses 5 minutes. The config file is unchanged.

**Reload from file:**

```
piri client admin config reload
```

The node returns to 30 seconds. The file wins; runtime overrides do not survive a reload.

**Edit the file, then reload:**

```toml
[pdp.aggregation.manager]
poll_interval = "5m"
```

```
piri client admin config reload
```

The node uses 5 minutes. The file still wins—it just agrees with you now.

**Runtime override with `--persist`:**

```
piri client admin config set pdp.aggregation.manager.poll_interval 10m --persist
```

The node uses 10 minutes and the config file is updated to match. Future reloads will preserve the change.

## Usage

```
piri client admin config [command]
```

## Subcommands

### [list](list.md)

List all dynamic configuration values.

### [get](get.md)

Get a specific configuration value.

### [set](set.md)

Set a configuration value.

### [reload](reload.md)

Reload configuration from file.
