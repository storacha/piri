# Payment Rail Dynamics and Status Changes

This document explains how payment rails are affected by various operations and when you can expect the `payments status` command output to change.

## Table of Contents

1. [Overview](#overview)
2. [When Adding Roots](#when-adding-roots)
3. [Proving Period Transitions](#proving-period-transitions)
4. [What Exceeding Allowances Means](#what-exceeding-allowances-means)
5. [When Status Output Changes](#when-status-output-changes)
6. [Settlement Behavior](#settlement-behavior)
7. [Storage Provider Perspective: When Payment Rails Can't Update](#storage-provider-perspective-when-payment-rails-cant-update)

---

## Overview

The payment rail system is **dynamic** and **automatically adjusts** based on:
- Data size (leaf count)
- Time elapsed (automatic lockup settlement)
- Proving period transitions

However, changes are **constrained** by:
- Approved rate and lockup allowances
- Deposited funds in the payer account

---

## When Adding Roots

### Flow

1. **Roots Are Added** (`piecesAdded` callback)
   - Location: `FilecoinWarmStorageService.sol:717`
   - Pieces are stored with their metadata
   - **No immediate payment changes**

2. **Proving Period Is Initialized or Transitions** (`nextProvingPeriod`)
   - Location: `FilecoinWarmStorageService.sol:853`
   - Called after roots are added (initialization) OR during proving period transitions
   - Lines 874 and 930: **ALWAYS calls `updatePaymentRates(dataSetId, leafCount)`**

3. **Payment Rate Is Updated** (`updatePaymentRates`)
   - Location: `FilecoinWarmStorageService.sol:1127`
   ```solidity
   uint256 totalBytes = leafCount * BYTES_PER_LEAF;
   uint256 newStorageRatePerEpoch = _calculateStorageRate(totalBytes);
   payments.modifyRailPayment(pdpRailId, newStorageRatePerEpoch, 0);
   ```
   - Automatically recalculates payment rate based on **current data size**
   - Emits `RailRateUpdated` event

### What Happens If You Exceed Configured Allowances

The `modifyRailPayment` transaction will **REVERT** if any of these conditions fail:

#### A. Payer Account Not Fully Settled
**Location:** `Payments.sol:989-994`
```solidity
bool isSettled = isAccountLockupFullySettled(payer);
require(
    isSettled || newRate == oldRate,
    "LockupNotSettledRateChangeNotAllowed"
);
```
- **Meaning:** If the payer hasn't deposited enough funds to cover existing lockup, the rate CANNOT be increased
- **Fix:** Deposit more funds and wait for settlement (or call a method that triggers settlement)

#### B. Rate Allowance Exceeded
**Location:** `Payments.sol:1621-1624` (called from `updateOperatorRateUsage`)
```solidity
require(
    approval.rateUsage + rateIncrease <= approval.rateAllowance,
    "OperatorRateAllowanceExceeded"
);
```
- **Meaning:** If the new rate per epoch exceeds your approved rate allowance, the transaction REVERTS
- **Fix:** Run `payments calculate --size <new_size>` to get new allowances, then run `payments approve-service` or use `increaseOperatorApproval`

#### C. Lockup Allowance Exceeded
**Location:** `Payments.sol:1638-1641` (called from `updateOperatorLockupUsage`)
```solidity
require(
    approval.lockupUsage + lockupIncrease <= approval.lockupAllowance,
    "OperatorLockupAllowanceExceeded"
);
```
- **Meaning:** If the new total lockup (rate × lockup period) exceeds your approved lockup allowance, the transaction REVERTS
- **Fix:** Run `payments calculate --size <new_size>` to get new allowances, then increase your approval

---

## Proving Period Transitions

### Every Proving Period Transition Affects the Rail

**Location:** `FilecoinWarmStorageService.sol:853` (`nextProvingPeriod`)

This function is called:
1. **On initialization** - When the first proving period starts (line 874)
2. **Every proving period transition** - When moving to the next proving period (line 930)

**It ALWAYS calls:**
```solidity
updatePaymentRates(dataSetId, leafCount);  // Line 874 or 930
```

### What Happens During updatePaymentRates

**Location:** `FilecoinWarmStorageService.sol:1127`

```solidity
uint256 totalBytes = leafCount * BYTES_PER_LEAF;
uint256 newStorageRatePerEpoch = _calculateStorageRate(totalBytes);
payments.modifyRailPayment(pdpRailId, newStorageRatePerEpoch, 0);
```

This calls `modifyRailPayment` with the calculated rate based on **current** leaf count.

### Lockup Settlement Happens Even If Rate Doesn't Change

**Location:** `Payments.sol:965` (`modifyRailPayment`)

The function has the `settleAccountLockupBeforeAndAfterForRail` modifier (line 970), which:
1. Calls `settleAccountLockup(rail.token, rail.from, payer)` **BEFORE** the function (line 258)
2. Calls `settleAccountLockup(rail.token, rail.from, payer)` **AFTER** the function (line 265)

**Location:** `Payments.sol:1514` (`settleAccountLockup`)

```solidity
uint256 elapsedTime = currentEpoch - account.lockupLastSettledAt;
uint256 additionalLockup = account.lockupRate * elapsedTime;

if (account.funds >= account.lockupCurrent + additionalLockup) {
    account.lockupCurrent += additionalLockup;
    account.lockupLastSettledAt = currentEpoch;
}
```

**This means:**
- Even if `newRate == oldRate`, the lockup settlement still happens
- `lockupCurrent` increases by `lockupRate × elapsedTime` (if sufficient funds)
- `lockupLastSettledAt` is updated to current epoch

### Rate Change Only If Leaf Count Changed

**Location:** `Payments.sol:1046` (`enqueueRateChange`)

```solidity
if (newRate == oldRate || rail.settledUpTo == block.number) {
    return;  // Returns early if no change
}
```

- If the leaf count hasn't changed, `newRate == oldRate`, and the function returns early
- No rate change is enqueued
- But lockup settlement still occurred (via the modifier)

---

## What Exceeding Allowances Means

### Scenario: You're Configured for 1 TiB, But Add 2 TiB of Data

**What Happens:**
1. `piecesAdded` callback stores the pieces
2. `nextProvingPeriod` is called (either immediately for init, or at next period)
3. `updatePaymentRates` calculates new rate for 2 TiB of data
4. `modifyRailPayment` is called with the higher rate
5. **Transaction REVERTS** with one of:
   - `"LockupNotSettledRateChangeNotAllowed"` - Payer account not fully settled
   - `"OperatorRateAllowanceExceeded"` - New rate exceeds your rate allowance
   - `"OperatorLockupAllowanceExceeded"` - New lockup exceeds your lockup allowance

**How to Add More Than Configured:**
1. Calculate new allowances:
   ```bash
   ./service-operator payments calculate --size 2TiB
   ```
2. Increase your operator approval:
   ```bash
   ./service-operator payments approve-service \
     --rate-allowance <new_rate> \
     --lockup-allowance <new_lockup> \
     --max-lockup-period <new_period>
   ```
3. Deposit more funds to cover the increased lockup:
   ```bash
   ./service-operator payments deposit --amount <additional_amount>
   ```
4. Then add the additional roots

---

## When Status Output Changes

The `payments status` command queries the **current on-chain state**. The output changes when:

### Immediately After These Operations:

#### 1. `deposit` Command
**Location:** `tools/service-operator/cmd/payments/deposit.go`

**Changes:**
- ✅ `Total funds` **increases**
- ✅ `Available funds` **increases**
- ❌ `Locked funds` - unchanged
- ❌ `Rate usage` - unchanged
- ❌ `Lockup usage` - unchanged

#### 2. `settle` Command (Manual Settlement)
**Location:** `tools/service-operator/cmd/payments/settle.go`

**Changes:**
- ✅ `Total funds` **decreases** (settled funds transferred to payee)
- ✅ `Locked funds` **decreases** (settled portion unlocked)
- ✅ `Available funds` - stays roughly the same (both decrease)
- ❌ `Rate usage` - **unchanged** (rail still active)
- ❌ `Lockup usage` - **unchanged** (rail still active)

#### 3. Adding Roots (If Successful and Data Size Increased)
**Triggered by:** `nextProvingPeriod` → `updatePaymentRates` → `modifyRailPayment`

**Changes:**
- ✅ `Locked funds` **increases** (both from settlement AND rate increase)
- ✅ `Available funds` **decreases**
- ✅ `Rate usage` **increases** (reflecting new data size)
- ✅ `Lockup usage` **increases** (reflecting new lockup requirements)
- ❌ `Total funds` - unchanged (no settlement to payee)

#### 4. Proving Period Transition (Even With No New Roots)
**Triggered by:** `nextProvingPeriod` → `updatePaymentRates` → `modifyRailPayment` → `settleAccountLockup`

**Changes:**
- ✅ `Locked funds` **increases** (time-based lockup accumulation)
- ✅ `Available funds` **decreases**
- ❌ `Rate usage` - unchanged (if no size change)
- ❌ `Lockup usage` - unchanged (if no size change)
- ❌ `Total funds` - unchanged (no settlement to payee)

**This happens automatically every proving period!**

#### 5. Terminating a Rail
**Location:** `Payments.sol:403` (`terminateRail`)

**Changes:**
- ✅ `Rate usage` **decreases** (rate capacity freed)
- ✅ `Lockup usage` - will decrease over time as rail winds down
- ❌ `Locked funds` - unchanged initially, decreases as rail is settled
- ❌ `Available funds` - unchanged initially, increases as lockup is released

---

## Settlement Behavior

### Automatic Lockup Settlement

**When It Happens:**
- Every time `modifyRailPayment`, `modifyRailLockup`, `deposit`, `withdraw`, or `terminateRail` is called
- Triggered by the `settleAccountLockupBeforeAndAfterForRail` or `settleAccountLockupBeforeAndAfter` modifiers

**What It Does:**
1. Calculates elapsed time: `elapsedTime = currentEpoch - lockupLastSettledAt`
2. Calculates additional lockup: `additionalLockup = lockupRate × elapsedTime`
3. If payer has sufficient funds: `lockupCurrent += additionalLockup`
4. Updates `lockupLastSettledAt = currentEpoch`

**Effect on Status:**
- `Locked funds` increases
- `Available funds` decreases
- `Total funds` unchanged

### Manual Payment Settlement

**How To Trigger:**
```bash
./service-operator payments settle --rail-id <id>
# or
./service-operator payments settle --all
```

**What It Does:**
1. Requires NETWORK_FEE (0.0013 FIL) to be sent with transaction
2. Calls validator (`validatePayment`) to check which epochs have valid PDP proofs
3. Only pays for proven epochs (epochs with valid proofs)
4. Transfers funds from payer's account to payee's account
5. Applies commission split if configured
6. Emits `RailSettled` event

**Effect on Status:**
- `Total funds` decreases (funds transferred out)
- `Locked funds` decreases (settled portion unlocked)
- `Available funds` stays roughly same
- `Rate usage` **unchanged** (rail still active)
- `Lockup usage` **unchanged** (rail still active)

---

## Storage Provider Perspective: When Payment Rails Can't Update

### What Happens When Data Exceeds Configured Allowances

This section explains the operational scenario from a storage provider's perspective when roots are added but the payer's allowances or funds can't support the increased data size.

#### The Scenario

1. **Initial State**: Storage provider is approved and serving a client with 1 TiB of data configured
2. **Client adds more roots**: Client adds 2 TiB of data (now 3 TiB total)
3. **Roots are stored successfully**: `piecesAdded` callback stores the new roots (no payment check at this point)
4. **Proving period attempts to advance**: Automated `NextProvingPeriodTask` triggers when current period is ending
5. **Payment rail update fails**: Transaction reverts with one of three errors (see "What Happens If You Exceed Configured Allowances" above)

#### Who Calls `nextProvingPeriod`?

**The storage provider's automated system calls it**, not the client:

- **Automatic trigger**: `NextProvingPeriodTask` (see `pkg/pdp/tasks/next_pdp.go:48-87`)
- **Triggered when**: `(prove_at_epoch + challenge_window) <= current_block_height`
- **Transaction sender**: The storage provider's address (from keystore)
- **Transaction content**: Calls `PDPVerifier.nextProvingPeriod(dataSetId, challengeEpoch, extraData)`
- **Callback flow**: PDPVerifier → FilecoinWarmStorageService → updatePaymentRates → modifyRailPayment

#### What Happens When Transaction Reverts

**Immediate Effects:**
- ❌ Transaction fails on-chain
- ❌ Proving period does NOT advance
- ❌ `provingDeadlines[dataSetId]` remains at previous deadline
- ❌ `provenThisPeriod[dataSetId]` is NOT reset
- ❌ Payment rate is NOT updated
- ✅ Roots ARE already in PDPVerifier (added during `piecesAdded`)

**Operational Impact:**

1. **Storage provider is stuck in idle state**: Cannot advance to next proving period until payment rail update succeeds
2. **Automated task keeps trying**: System will attempt `nextProvingPeriod` again in future blocks (after task retry logic)
3. **Storage provider cannot submit new proofs**: `provenThisPeriod` flag remains `true`, so any proof submission would revert with "ProofAlreadySubmitted"
4. **No new challenge is issued**: The stuck proving period means no new challenge is generated
5. **New roots are not being proven yet**: The added roots are in the data set, but not yet part of any proving challenge

#### What Is the Storage Provider Expected To Prove?

**Current State (stuck):**
- Storage provider already proved the current period (before roots were added)
- `provenThisPeriod[dataSetId] = true`
- **Cannot submit another proof** - would revert with `Errors.ProofAlreadySubmitted`
- **Not being asked to prove anything** - stuck waiting in idle state

**After `nextProvingPeriod` succeeds:**
- `provenThisPeriod` is reset to `false`
- New challenge is generated for the next period
- New challenge will include ALL roots (old + newly added)
- Storage provider can now prove the new challenge

#### Resolution Steps

**For the CLIENT (payer) to fix:**

1. **Calculate new allowances** for increased data size:
   ```bash
   ./service-operator payments calculate --size 3TiB
   ```

2. **Increase operator approval** with new allowances:
   ```bash
   ./service-operator payments approve-service \
     --rate-allowance <new_rate> \
     --lockup-allowance <new_lockup> \
     --max-lockup-period <new_period>
   ```

3. **Deposit additional funds** to cover increased lockup:
   ```bash
   ./service-operator payments deposit --amount <additional_amount>
   ```

**After client fixes allowances:**
- Next `nextProvingPeriod` attempt will succeed
- Proving period will advance
- New challenge will include all roots (including newly added ones)
- Payment rate will reflect actual data size

#### Key Timeline

```
Epoch 1000: Client has 1 TiB, paying for 1 TiB
Epoch 1800: SP proves current period successfully
            → provenThisPeriod = true
Epoch 1500: Client adds 2 TiB of roots (piecesAdded succeeds, total now 3 TiB)
Epoch 2880: Current proving deadline passes, nextProvingPeriod called
            → Tries to set provenThisPeriod = false
            → Tries updatePaymentRates for 3 TiB
            → modifyRailPayment reverts: "OperatorRateAllowanceExceeded"
            → Transaction reverts, provenThisPeriod stays true
            → Proving period STUCK
Epoch 2900: SP tries to prove again → reverts "ProofAlreadySubmitted"
            → SP is now idle, waiting
Epoch 3000: NextProvingPeriodTask retries → still fails (allowances unchanged)
Epoch 3500: Client increases allowances and deposits funds
Epoch 3600: NextProvingPeriodTask retries → SUCCESS
            → provenThisPeriod reset to false
            → Proving period advances to epoch 5760
            → New challenge generated for all 3 TiB
            → Payment rate now correct for 3 TiB
Epoch 5400: SP proves new challenge → provenThisPeriod = true
            → Normal operation resumes
```

#### Important Considerations

1. **No immediate feedback**: When client adds roots, there's no immediate error - `piecesAdded` succeeds
2. **Failure is delayed**: Error only occurs when proving period tries to advance (could be hours/days later depending on proving period length)
3. **Storage provider is stuck idle**: SP cannot submit proofs (reverts), not being asked to prove anything, just waiting
4. **Client must proactively manage**: Client should increase allowances BEFORE adding roots that would exceed configured capacity
5. **Grace period**: Time between root addition and proving period advancement provides window for client to fix allowances before proving schedule gets stuck
6. **No faults recorded during stuck period**: Since SP already proved the stuck period, no faults are recorded even while waiting

#### Monitoring Recommendations

**For Storage Providers:**
- Monitor `NextProvingPeriodTask` failures in logs
- Alert when same task keeps failing
- Communicate with client when payment rail updates fail

**For Clients (Payers):**
- Run `payments status` before adding large amounts of data
- Ensure `Available rate` and `Available lockup` have headroom
- Increase allowances BEFORE adding roots if approaching limits

---

## Summary: Expected Status Changes Over Time

### Normal Operation (No New Roots Added)

**Every Proving Period (~2880 epochs / 1 day):**
- `nextProvingPeriod` is called
- `modifyRailPayment` is called (even with same rate)
- Lockup settlement happens
- **Result:** `Locked funds` increases, `Available funds` decreases

**When You Run Settlement:**
- `settle` command transfers proven epochs' payments
- **Result:** `Total funds` and `Locked funds` decrease

### When Adding New Roots

**If Within Configured Allowances:**
- Roots added successfully
- Next `nextProvingPeriod` call triggers rate update
- **Result:** `Locked funds`, `Rate usage`, and `Lockup usage` all increase

**If Exceeding Configured Allowances:**
- Roots added successfully
- Next `nextProvingPeriod` call **REVERTS** when trying to update rate
- **Action Required:** Increase allowances and deposit more funds before next proving period

---

## Key Takeaways

1. **Payment rails are dynamic** - they automatically adjust based on data size
2. **Lockup settlement happens automatically** - every proving period, even if rate doesn't change
3. **Status changes every proving period** - `Locked funds` increases as time passes
4. **Adding roots can fail at proving period** - if new size exceeds allowances
5. **Settlement is manual** - you must call `settle` to transfer funds from payer to payee
6. **Settlement only pays for proven epochs** - validator checks PDP proofs before paying

---

## References

### Key Contract Functions

- `FilecoinWarmStorageService.sol:853` - `nextProvingPeriod`
- `FilecoinWarmStorageService.sol:1127` - `updatePaymentRates`
- `Payments.sol:965` - `modifyRailPayment`
- `Payments.sol:1514` - `settleAccountLockup`
- `Payments.sol:1617` - `updateOperatorRateUsage`
- `Payments.sol:1632` - `updateOperatorLockupUsage`
- `Payments.sol:1172` - `settleRail`

### CLI Commands

- `service-operator payments status` - View current payment rail status
- `service-operator payments calculate --size <size>` - Calculate allowances for size
- `service-operator payments approve-service` - Set operator allowances
- `service-operator payments deposit --amount <amount>` - Deposit funds
- `service-operator payments settle --rail-id <id>` - Settle specific rail
- `service-operator payments settle --all` - Settle all rails
