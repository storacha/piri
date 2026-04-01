# repair

Repair a proof set by reconciling stuck roots with on-chain state.

When roots are added to the blockchain but never recorded in the database—typically due to Lotus state loss or similar misfortune—the PDP proving pipeline grinds to a halt.
This command fetches all active pieces from the blockchain contract, compares them with the database, and repairs any gaps using metadata from pending root additions. 

## Usage

```
piri client pdp proofset repair [flags]
```

## Flags

| Flag                 | Description                                                                      |
|----------------------|----------------------------------------------------------------------------------|
| `--proofset-id <id>` | The proof set ID to repair (can also be set via config file as `ucan.proof_set`) |

## Example

```bash
piri client pdp proofset repair --proofset-id 123
```

When repairs are performed:

```json
{
  "totalOnChain": 42,
  "totalInDB": 40,
  "totalRepaired": 2,
  "totalUnrepaired": 0,
  "repairedEntries": [
    {
      "rootCid": "bafk...",
      "rootId": 123,
      "subrootsRepaired": 1
    }
  ],
  "unrepairedEntries": []
}
```

When the chain and database are already in agreement (no repair needed):

```json
{
  "totalOnChain": 42,
  "totalInDB": 42,
  "totalRepaired": 0,
  "totalUnrepaired": 0,
  "repairedEntries": [],
  "unrepairedEntries": []
}
```

## Output Fields

| Field               | Description                                                                          |
|---------------------|--------------------------------------------------------------------------------------|
| `totalOnChain`      | Number of active pieces on the blockchain                                            |
| `totalInDB`         | Number of roots in the database                                                      |
| `totalRepaired`     | Number of roots successfully repaired                                                |
| `totalUnrepaired`   | Number of roots that could not be repaired (metadata missing from pending additions) |
| `repairedEntries`   | Details of each repaired root, including root CID, root ID, and subroots repaired    |
| `unrepairedEntries` | Details of roots that could not be repaired, with reason                             |
