# Validate Your Piri Setup

You are now running a unified Piri node with both UCAN and PDP functionality!

## Prerequisites

Before validating, ensure you have:
- ✅ [Setup Piri Node](../guides/piri-server.md)

## Validation Steps

### 1. Test Data Reception

Visit https://staging.delegator.storacha.network/test-storage to test your setup. The process involves three main steps:

#### Step 1: Enter Your Node Details

![Test Storage Step 1](../images/test-storage-step1.png)

1. **Enter your node's DID**: 
   - Format must start with `did:key:` followed by your identifier
   - Example: `did:key:z6MksyRCPWoXvMj8sUzuHiQ4pFkSawkKRz2eh1TALNEG6s3e`

2. **Enter your Piri Node URL**:
   - This is the full URL where your Piri node can be reached
   - It should respond with your DID
   - Example: `https://piri.example.com`

3. **Click Continue** to proceed to the next step

#### Step 2: Submit Delegation Proof

![Test Storage Step 2](../images/test-storage-step2.png)

1. **Generate a delegation proof**:
   - Click the "How to generate" link for detailed instructions
   - Use the piri CLI with the command shown in the interface:
   ```bash
   # Using the service.pem file you saved during setup (either PEM or JSON format):
   piri delegate generate \
     --key-file=service.pem \
     --client-did=did:key:z6MkuQ8PfSMrzXCwZkbQv662nZC4FGGm1aucbH256HXXZyxo
   ```

2. **Paste your delegation proof**:
   - Copy the output from the command above
   - Paste it into the "Delegation Proof" text area

3. **Click "Test Storage"** to run the test

If successful, you'll receive confirmation that your Piri setup is correctly receiving data from the Storacha Network.

### 2. Inspect Your Proof Set

After successfully testing storage, you can monitor your proof sets by visiting the [PDPscan Proof Set Inspector](inspect-proof-set.md).

---

Congratulations! Your Piri setup is complete and ready to receive data from the Storacha Network.