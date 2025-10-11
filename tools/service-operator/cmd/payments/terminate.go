package payments

// TODO: Implement payments terminate command to terminate payment rails
//
// ## Research Findings:
// - Payments contract has `terminateRail(railId)` function - available in bindings
// - Access control: Can be called by the **payer** (if lockup fully settled) OR the **operator**
// - No terminate command exists yet - need to create it
//
// ## Implementation Plan:
//
// 1. Create command structure similar to settle.go:
//    - Support `--rail-id <id>` to terminate a specific rail
//    - Support `--all` to terminate all rails
//    - Support `--rail-ids 1,2,3,4,5,6` to terminate multiple specific rails
//    - Load private key, create transaction auth
//    - Call contract's `TerminateRail` function
//    - Display success/failure for each rail
//
// 2. Add `TerminateRail` helper to `internal/contract/settlement.go`:
//    - Function signature: `TerminateRail(ctx, rpcURL, paymentsAddress, auth, railID)`
//    - Create Payments transactor binding
//    - Call `TerminateRail(auth, railID)`
//    - Wait for transaction to be mined
//    - Return transaction hash and status
//
// 3. Register command in `cmd/payments/root.go`:
//    - Add `Cmd.AddCommand(terminateCmd)`
//
// ## Usage Examples:
// ```bash
// # Terminate a specific rail
// service-operator payments terminate --rail-id 1
//
// # Terminate multiple specific rails
// service-operator payments terminate --rail-ids 1,2,3,4,5,6
//
// # Terminate all rails
// service-operator payments terminate --all
// ```
//
// ## Contract Details:
// - Function: terminateRail(uint256 railId)
// - Access: Payer (if lockup fully settled) OR Operator
// - Sets rail.endEpoch to terminate the rail
// - Emits RailTerminated event
