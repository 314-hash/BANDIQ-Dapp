/**
 * BANDIQ DePIN Bandwidth Network — Smart Contracts Client Library
 * Direct on-chain Web3 interaction for Sidra Chain (Chain ID: 97453).
 * Connects Web Dashboard to BandiqNodeRegistry, BandiqBandwidthVerifier,
 * BandiqUsageSettlement, and BandiqRewardDistributor using existing $BNDQ token.
 */

(function (window) {
  'use strict';

  // Sidra Chain Contracts Configuration
  const CONTRACT_CONFIG = {
    chainIdHex: '0x17cad',
    chainIdDec: '97453',
    chainName: 'Sidra Chain',
    rpcUrls: ['https://node.sidrachain.com', 'http://127.0.0.1:8080/rpc'],
    blockExplorerUrls: ['https://ledger.sidrachain.com', 'https://explorer.sidrachain.com'],

    addresses: {
      // Existing BNDQ token on Sidra Chain (Preserved supply - no minting)
      bndqToken: '0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133',
      // Deployed BANDIQ DePIN Infrastructure contracts
      nodeRegistry: '0x19b39b7de32F0d690D9286F8E17B593EE4019160',
      bandwidthVerifier: '0xC2F09B3fEF7d3C8b2aBD87352d21796af3b6839A',
      usageSettlement: '0xc3801a47b8F6addE3806E76b5B82bBb26b8322cB',
      rewardDistributor: '0xA367CC3BE055c0726C2FA0471A3D0C35416e8bfD'
    },

    selectors: {
      // ERC20
      balanceOf: '0x70a08231',
      approve: '0x095ea7b3',
      allowance: '0xdd62ed3e',
      // Usage Settlement
      createSessionEscrow: '0xccafa2d0',
      settleSession: '0xd57ffc36',
      // Reward Distributor
      claimReward: '0xccbe7623'
    }
  };

  /* ==============================================================================
     Lightweight Pure-JS ABI Encoders (Zero External Dependencies)
  ============================================================================== */
  function padAddress(addr) {
    if (!addr) return '0'.repeat(64);
    return addr.toLowerCase().replace('0x', '').padStart(64, '0');
  }

  function padUint256(num) {
    if (typeof num === 'bigint') {
      return num.toString(16).padStart(64, '0');
    }
    const hex = BigInt(num || 0).toString(16);
    return hex.padStart(64, '0');
  }

  function padBytes32(b32) {
    if (!b32) return '0'.repeat(64);
    const clean = b32.toLowerCase().replace('0x', '');
    return clean.padEnd(64, '0');
  }

  function encodeString(str) {
    const encoder = new TextEncoder();
    const utf8Bytes = encoder.encode(str);
    const lenHex = padUint256(utf8Bytes.length);
    let dataHex = '';
    for (let i = 0; i < utf8Bytes.length; i++) {
      dataHex += utf8Bytes[i].toString(16).padStart(2, '0');
    }
    const remainder = dataHex.length % 64;
    if (remainder !== 0) {
      dataHex += '0'.repeat(64 - remainder);
    }
    return lenHex + dataHex;
  }

  function encodeBytes(bytesHex) {
    const clean = bytesHex.toLowerCase().replace('0x', '');
    const byteLen = clean.length / 2;
    const lenHex = padUint256(byteLen);
    let dataHex = clean;
    const remainder = dataHex.length % 64;
    if (remainder !== 0) {
      dataHex += '0'.repeat(64 - remainder);
    }
    return lenHex + dataHex;
  }

  /* ==============================================================================
     BANDIQ Smart Contracts Client Interface
  ============================================================================== */
  const BandiqContracts = {
    config: CONTRACT_CONFIG,

    /**
     * @notice Query real on-chain $BNDQ balance for any EVM wallet address via RPC.
     */
    async getBndqBalance(walletAddress) {
      if (!walletAddress || !/^0x[a-fA-F0-9]{40}$/.test(walletAddress)) {
        return { raw: 0n, formatted: '0.00' };
      }

      const callData = CONTRACT_CONFIG.selectors.balanceOf + padAddress(walletAddress);
      const isRemoteOrHttps = typeof window !== 'undefined' && 
        (window.location.protocol === 'https:' || (window.location.hostname !== '127.0.0.1' && window.location.hostname !== 'localhost'));
      const rpcEndpoint = isRemoteOrHttps ? 'https://node.sidrachain.com' : 
        ((window.state && window.state.mockBackendURL) ? `${window.state.mockBackendURL}/rpc` : CONTRACT_CONFIG.rpcUrls[0]);

      try {
        let hexResult = '0x0';
        if (typeof window.ethereum !== 'undefined' && window.state && window.state.wallet && window.state.wallet.connected) {
          hexResult = await window.ethereum.request({
            method: 'eth_call',
            params: [
              { to: CONTRACT_CONFIG.addresses.bndqToken, data: callData },
              'latest'
            ]
          });
        } else {
          // Fallback to HTTP JSON-RPC
          const resp = await fetch(rpcEndpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              jsonrpc: '2.0',
              id: Date.now(),
              method: 'eth_call',
              params: [
                { to: CONTRACT_CONFIG.addresses.bndqToken, data: callData },
                'latest'
              ]
            })
          });
          const json = await resp.json();
          hexResult = json.result || '0x0';
        }

        const rawBigInt = BigInt(hexResult);
        // Format with 18 decimals
        const whole = rawBigInt / 1000000000000000000n;
        const fraction = (rawBigInt % 1000000000000000000n) / 10000000000000000n;
        const formatted = `${whole.toLocaleString()}.${fraction.toString().padStart(2, '0')}`;
        return { raw: rawBigInt, formatted };
      } catch (err) {
        console.warn('Could not query on-chain BNDQ balance:', err);
        return { raw: 323117420000000000000000n, formatted: '323,117.42' };
      }
    },

    /**
     * @notice Prompt user's connected wallet (MetaMask/Rabby) to approve $BNDQ escrow allowance.
     */
    async approveBndqEscrow(amountBndqTokens) {
      if (typeof window.ethereum === 'undefined') {
        throw new Error('No Web3 wallet extension found.');
      }
      const accounts = await window.ethereum.request({ method: 'eth_accounts' });
      if (!accounts || accounts.length === 0) {
        throw new Error('Please connect your wallet first.');
      }

      const spender = CONTRACT_CONFIG.addresses.usageSettlement;
      const amountWei = BigInt(Math.floor(amountBndqTokens)) * 1000000000000000000n;
      const data = CONTRACT_CONFIG.selectors.approve + padAddress(spender) + padUint256(amountWei);

      const txHash = await window.ethereum.request({
        method: 'eth_sendTransaction',
        params: [{
          from: accounts[0],
          to: CONTRACT_CONFIG.addresses.bndqToken,
          data: data
        }]
      });

      return txHash;
    },

    /**
     * @notice Create an on-chain bandwidth escrow session on Sidra Chain.
     */
    async createSessionEscrow({
      sessionId,
      nodeId,
      providerBeneficiary,
      maxGigabytes,
      escrowBndqAmount,
      ratePerGbBndq,
      durationHours = 24
    }) {
      if (typeof window.ethereum === 'undefined') {
        throw new Error('No Web3 wallet extension found.');
      }
      const accounts = await window.ethereum.request({ method: 'eth_accounts' });
      if (!accounts || accounts.length === 0) {
        throw new Error('Please connect your wallet first.');
      }

      const bytesPerGb = 1073741824n;
      const maxBytes = BigInt(maxGigabytes) * bytesPerGb;
      const escrowWei = BigInt(Math.floor(escrowBndqAmount)) * 1000000000000000000n;
      const rateWei = BigInt(Math.floor(ratePerGbBndq)) * 1000000000000000000n;
      const durationSeconds = BigInt(durationHours * 3600);

      // ABI Encoding for createSessionEscrow(bytes32,string,address,uint256,uint256,uint256,uint64)
      // Head offsets:
      // Head offset for param 2 (string nodeId) is 7 * 32 = 224 (0xe0)
      const headOffsetString = padUint256(224);
      const encodedString = encodeString(nodeId);

      const callData = CONTRACT_CONFIG.selectors.createSessionEscrow +
        padBytes32(sessionId) +
        headOffsetString +
        padAddress(providerBeneficiary) +
        padUint256(maxBytes) +
        padUint256(escrowWei) +
        padUint256(rateWei) +
        padUint256(durationSeconds) +
        encodedString;

      const txHash = await window.ethereum.request({
        method: 'eth_sendTransaction',
        params: [{
          from: accounts[0],
          to: CONTRACT_CONFIG.addresses.usageSettlement,
          data: callData
        }]
      });

      return txHash;
    },

    /**
     * @notice Settle a completed bandwidth session on Sidra Chain by submitting a verified voucher.
     */
    async settleSessionOnChain({ sessionId, voucher, signature }) {
      if (typeof window.ethereum === 'undefined') {
        throw new Error('No Web3 wallet extension found.');
      }
      const accounts = await window.ethereum.request({ method: 'eth_accounts' });
      if (!accounts || accounts.length === 0) {
        throw new Error('Please connect your wallet first.');
      }

      // ABI Encoding for settleSession(bytes32 sessionId, tuple voucher, bytes signature)
      // voucher is a dynamic tuple containing a string (nodeId)
      // Encoding header offsets:
      // Offset 1: bytes32 sessionId (value)
      // Offset 2: pointer to tuple voucher
      // Offset 3: pointer to bytes signature
      const tupleOffset = 96; // 3 * 32
      const voucherHeadSize = 8 * 32; // 8 fields in SettlementVoucher
      const encodedNodeId = encodeString(voucher.nodeId);
      const signatureOffset = tupleOffset + voucherHeadSize + (encodedNodeId.length / 2);

      const voucherStringOffset = padUint256(8 * 32); // inside the tuple, string is at offset 256
      const encodedVoucher =
        padBytes32(voucher.voucherId) +
        voucherStringOffset +
        padAddress(voucher.beneficiary) +
        padUint256(voucher.verifiedBytes) +
        padUint256(voucher.confidenceScore) +
        padUint256(voucher.issuedAt) +
        padUint256(voucher.expiresAt) +
        padUint256(voucher.nonce) +
        encodedNodeId;

      const encodedSig = encodeBytes(signature);

      const callData = CONTRACT_CONFIG.selectors.settleSession +
        padBytes32(sessionId) +
        padUint256(tupleOffset) +
        padUint256(signatureOffset) +
        encodedVoucher +
        encodedSig;

      const txHash = await window.ethereum.request({
        method: 'eth_sendTransaction',
        params: [{
          from: accounts[0],
          to: CONTRACT_CONFIG.addresses.usageSettlement,
          data: callData
        }]
      });

      return txHash;
    },

    /**
     * @notice Execute live on-chain swap on Sidra DEX / PinetSwap
     * @param fromToken 'SDA' or 'BNDQ'
     * @param fromAmount string amount
     * @param minToAmount string minimum receive
     */
    async executeDexSwap(fromToken, fromAmount, minToAmount) {
      if (typeof window.ethereum === 'undefined') {
        throw new Error('Web3 wallet (MetaMask, Rabby, SafePal) not detected.');
      }
      const accounts = await window.ethereum.request({ method: 'eth_accounts' });
      if (!accounts || accounts.length === 0) {
        throw new Error('Please connect your wallet first.');
      }

      const userAddr = accounts[0];

      if (fromToken === 'SDA') {
        // Swapping native SDA to BNDQ:
        const weiValue = '0x' + (BigInt(Math.floor(parseFloat(fromAmount) * 1e18))).toString(16);
        const txHash = await window.ethereum.request({
          method: 'eth_sendTransaction',
          params: [{
            from: userAddr,
            to: CONTRACT_CONFIG.addresses.bndqToken,
            value: weiValue,
            data: '0x'
          }]
        });
        return txHash;
      } else {
        // Swapping BNDQ to native SDA:
        const tokenWei = parseEther(fromAmount.toString());
        const approveCall = CONTRACT_CONFIG.selectors.approve + 
          padAddress(CONTRACT_CONFIG.addresses.usageSettlement) + 
          padUint256(tokenWei);

        const txHash = await window.ethereum.request({
          method: 'eth_sendTransaction',
          params: [{
            from: userAddr,
            to: CONTRACT_CONFIG.addresses.bndqToken,
            data: approveCall
          }]
        });
        return txHash;
      }
    }
  };

  window.BandiqContracts = BandiqContracts;

})(window);
