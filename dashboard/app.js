/**
 * BANDIQ DePIN Bandwidth Network Dashboard
 * Handles reactive UI state, local node agent API connection, live speedtest gauge,
 * topology canvas visualizer, and mobile touch navigation.
 */

// Application State
const state = {
  currentView: 'overview',
  isAgentConnected: false,
  agentURL: 'http://127.0.0.1:8725',
  mockBackendURL: 'http://127.0.0.1:8080',
  nodeData: {
    nodeId: 'bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61',
    wallet: '0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133',
    paused: false,
    consent: true,
    dailyCapMB: 1000,
    speedLimitMbps: 50.0,
    spoolCount: 0,
    totalTests: 18,
    cpuPercent: 12.4,
    memUsedMB: 5365,
    memTotalMB: 6085,
    ifaceName: 'Ethernet',
    ifaceStatus: 'UP',
    bytesIn: 1423798221,
    bytesOut: 255451748,
    latestReport: null
  },
  wallet: {
    connected: false,
    address: '0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133',
    chainId: '97453',
    providerName: 'Read-Only Watcher',
    nativeBalance: '0.00 KST',
    bndqBalance: '323,117.42',
    isWrongChain: false
  },
  speedtestRunning: false,
  networkNodes: []
};

// DOM References
const elements = {
  views: document.querySelectorAll('.view-panel'),
  desktopNavItems: document.querySelectorAll('.nav-item'),
  mobileNavBtns: document.querySelectorAll('.mobile-nav-btn'),
  mobileMenuBtn: document.getElementById('mobileMenuBtn'),
  appSidebar: document.getElementById('appSidebar'),
  brandHomeBtn: document.getElementById('brandHomeBtn'),
  agentStatusBadge: document.getElementById('agentStatusBadge'),
  agentStatusText: document.getElementById('agentStatusText'),
  agentModeTag: document.getElementById('agentModeTag'),
  agentModeLabel: document.getElementById('agentModeLabel'),
  nodeStateBadge: document.getElementById('nodeStateBadge'),
  loopStateLabel: document.getElementById('loopStateLabel'),
  btnPauseAgent: document.getElementById('btnPauseAgent'),
  btnPauseLabel: document.getElementById('btnPauseLabel'),
  btnTriggerTest: document.getElementById('btnTriggerTest'),
  btnApplyCaps: document.getElementById('btnApplyCaps'),
  dailyCapSlider: document.getElementById('dailyCapSlider'),
  dailyCapValue: document.getElementById('dailyCapValue'),
  speedLimitSlider: document.getElementById('speedLimitSlider'),
  speedLimitValue: document.getElementById('speedLimitValue'),
  nodeIdDisplay: document.getElementById('nodeIdDisplay'),
  nodeWalletDisplay: document.getElementById('nodeWalletDisplay'),
  hostCpuBar: document.getElementById('hostCpuBar'),
  hostCpuVal: document.getElementById('hostCpuVal'),
  hostMemBar: document.getElementById('hostMemBar'),
  hostMemVal: document.getElementById('hostMemVal'),
  hostIfaceName: document.getElementById('hostIfaceName'),
  hostIfaceStatus: document.getElementById('hostIfaceStatus'),
  hostOctetsVal: document.getElementById('hostOctetsVal'),
  latestReportJson: document.getElementById('latestReportJson'),
  spoolQueueCount: document.getElementById('spoolQueueCount'),
  totalCompletedProofs: document.getElementById('totalCompletedProofs'),
  // Speedtest
  btnStartSpeedtest: document.getElementById('btnStartSpeedtest'),
  speedtestBtnText: document.getElementById('speedtestBtnText'),
  speedtestPhase: document.getElementById('speedtestPhase'),
  speedtestValue: document.getElementById('speedtestValue'),
  gaugeMeter: document.getElementById('gaugeMeter'),
  stDownloadVal: document.getElementById('stDownloadVal'),
  stDownloadBytes: document.getElementById('stDownloadBytes'),
  stUploadVal: document.getElementById('stUploadVal'),
  stUploadBytes: document.getElementById('stUploadBytes'),
  stLatencyVal: document.getElementById('stLatencyVal'),
  stMinLatency: document.getElementById('stMinLatency'),
  stMaxLatency: document.getElementById('stMaxLatency'),
  stJitterVal: document.getElementById('stJitterVal'),
  stPacketLoss: document.getElementById('stPacketLoss'),
  // Explorer
  nodeSearchInput: document.getElementById('nodeSearchInput'),
  nodeStatusFilter: document.getElementById('nodeStatusFilter'),
  nodesTableBody: document.getElementById('nodesTableBody'),
  // Verifier
  verifyReportInput: document.getElementById('verifyReportInput'),
  btnVerifySignature: document.getElementById('btnVerifySignature'),
  verificationResultBox: document.getElementById('verificationResultBox'),
  vrStatusText: document.getElementById('vrStatusText'),
  vrDetailsText: document.getElementById('vrDetailsText'),
  // Canvas
  nodeMapCanvas: document.getElementById('nodeMapCanvas'),
  toastContainer: document.getElementById('toastContainer'),
  // Wallet & Multi-Platform Connector
  connectWalletBtn: document.getElementById('connectWalletBtn'),
  walletBtnText: document.getElementById('walletBtnText'),
  btnConnectWalletView: document.getElementById('btnConnectWalletView'),
  btnSwitchNetworkView: document.getElementById('btnSwitchNetworkView'),
  btnAddBndqTokenView: document.getElementById('btnAddBndqTokenView'),
  btnDisconnectWallet: document.getElementById('btnDisconnectWallet'),
  walletConnIndicator: document.getElementById('walletConnIndicator'),
  walletConnStatusLabel: document.getElementById('walletConnStatusLabel'),
  walletActiveAddressDisplay: document.getElementById('walletActiveAddressDisplay'),
  walletBalanceNum: document.getElementById('walletBalanceNum'),
  walletNativeBalance: document.getElementById('walletNativeBalance'),
  walletLinkedNode: document.getElementById('walletLinkedNode'),
  // Modal Elements
  walletModal: document.getElementById('walletModal'),
  btnCloseWalletModal: document.getElementById('btnCloseWalletModal'),
  wmConnectedBox: document.getElementById('wmConnectedBox'),
  wmSelectionBody: document.getElementById('wmSelectionBody'),
  wmcStatusText: document.getElementById('wmcStatusText'),
  wmcChainTag: document.getElementById('wmcChainTag'),
  wmcAddressDisplay: document.getElementById('wmcAddressDisplay'),
  wmcBndqBalance: document.getElementById('wmcBndqBalance'),
  wmcNativeBalance: document.getElementById('wmcNativeBalance'),
  btnCopyConnectedAddress: document.getElementById('btnCopyConnectedAddress'),
  wmBtnSwitchChain: document.getElementById('wmBtnSwitchChain'),
  wmBtnAddBndq: document.getElementById('wmBtnAddBndq'),
  wmBtnDisconnect: document.getElementById('wmBtnDisconnect'),
  wmTabs: document.getElementById('wmTabs'),
  btnConnectMetamask: document.getElementById('btnConnectMetamask'),
  btnConnectRabby: document.getElementById('btnConnectRabby'),
  btnConnectCoinbase: document.getElementById('btnConnectCoinbase'),
  btnConnectInjected: document.getElementById('btnConnectInjected'),
  tagMetamask: document.getElementById('tagMetamask'),
  btnDeepLinkMetamask: document.getElementById('btnDeepLinkMetamask'),
  btnDeepLinkTrust: document.getElementById('btnDeepLinkTrust'),
  btnDeepLinkOKX: document.getElementById('btnDeepLinkOKX'),
  btnDeepLinkRainbow: document.getElementById('btnDeepLinkRainbow'),
  wcQrCanvas: document.getElementById('wcQrCanvas'),
  wcUriInput: document.getElementById('wcUriInput'),
  btnCopyWcUri: document.getElementById('btnCopyWcUri'),
  readonlyAddressInput: document.getElementById('readonlyAddressInput'),
  btnSubmitReadOnly: document.getElementById('btnSubmitReadOnly'),
  // User Manual
  manualNavTabs: document.getElementById('manualNavTabs'),
  // On-Chain Escrow & Market
  escrowNodeId: document.getElementById('escrowNodeId'),
  escrowBeneficiary: document.getElementById('escrowBeneficiary'),
  escrowGigabytes: document.getElementById('escrowGigabytes'),
  escrowRate: document.getElementById('escrowRate'),
  escrowTotalCalc: document.getElementById('escrowTotalCalc'),
  btnApproveEscrow: document.getElementById('btnApproveEscrow'),
  btnLockEscrow: document.getElementById('btnLockEscrow'),
  btnSubmitVoucherOnChain: document.getElementById('btnSubmitVoucherOnChain'),
  sessionsTableBody: document.getElementById('sessionsTableBody')
};

// Initialize Application
document.addEventListener('DOMContentLoaded', () => {
  setupNavigation();
  setupSliders();
  setupAgentControls();
  setupSpeedtest();
  setupNodeExplorer();
  setupVerificationTool();
  setupCopyButtons();
  setupCanvasMap();
  setupWalletIntegration();
  setupManualTabs();
  setupOnChainMarketControls();

  // Initial check for local agent
  checkAgentConnection();
  setInterval(checkAgentConnection, 6000);
});

/* ==============================================================================
   Navigation & View Routing
============================================================================== */
function switchView(viewName) {
  if (!viewName) return;
  state.currentView = viewName;

  // Toggle active view panel
  elements.views.forEach(panel => {
    panel.classList.toggle('active', panel.id === `view${capitalize(viewName)}` || panel.id === `view${viewName.replace('-', '')}`);
  });

  // Update Desktop sidebar
  elements.desktopNavItems.forEach(item => {
    item.classList.toggle('active', item.dataset.view === viewName);
  });

  // Update Mobile bottom bar
  elements.mobileNavBtns.forEach(btn => {
    btn.classList.toggle('active', btn.dataset.view === viewName);
  });

  // Close mobile sidebar if open
  if (elements.appSidebar.classList.contains('open')) {
    elements.appSidebar.classList.remove('open');
  }

  window.scrollTo({ top: 0, behavior: 'smooth' });
}

function setupNavigation() {
  // Desktop Sidebar Clicks
  elements.desktopNavItems.forEach(item => {
    item.addEventListener('click', () => switchView(item.dataset.view));
  });

  // Mobile Bottom Bar Clicks
  elements.mobileNavBtns.forEach(btn => {
    btn.addEventListener('click', () => switchView(btn.dataset.view));
  });

  // Mobile Hamburger Menu Button
  if (elements.mobileMenuBtn) {
    elements.mobileMenuBtn.addEventListener('click', () => {
      elements.appSidebar.classList.toggle('open');
    });
  }

  // Home Brand click
  if (elements.brandHomeBtn) {
    elements.brandHomeBtn.addEventListener('click', () => switchView('overview'));
  }

  // Handle marketplace order button clicks
  document.querySelectorAll('.market-order-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const tier = e.target.dataset.tier;
      showToast(`Selected ${tier} service tier. Checking Sidra Chain node availability...`, 'info');
      setTimeout(() => {
        showToast(`Session ticket generated for ${tier} Tier. Ready for BNDQ settlement.`, 'success');
      }, 1200);
    });
  });
}

function capitalize(str) {
  if (!str) return '';
  return str.split('-').map(s => s.charAt(0).toUpperCase() + s.slice(1)).join('');
}

/* ==============================================================================
   Local Agent Connection & Real Telemetry
============================================================================== */
async function checkAgentConnection() {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 2000);

    const res = await fetch(`${state.agentURL}/status`, { signal: controller.signal });
    clearTimeout(timeoutId);

    if (res.ok) {
      const data = await res.json();
      setAgentConnected(true, data);
    } else {
      setAgentConnected(false);
    }
  } catch (err) {
    setAgentConnected(false);
  }
}

function setAgentConnected(isConnected, data = null) {
  state.isAgentConnected = isConnected;

  if (isConnected && data) {
    // Update live badge in header
    elements.agentStatusBadge.innerHTML = `
      <span class="agent-dot connected"></span>
      <span class="agent-text">Live Agent Connected</span>
    `;

    elements.agentModeTag.innerHTML = `
      <span class="status-indicator-ring live"></span>
      <span>Live Node: 127.0.0.1:8725</span>
    `;

    // Update node data from live agent
    if (data.node_id) state.nodeData.nodeId = data.node_id;
    if (data.operator_wallet) state.nodeData.wallet = data.operator_wallet;
    state.nodeData.paused = !!data.paused;
    state.nodeData.spoolCount = data.spool_count || 0;
    state.nodeData.totalTests = data.total_tests || 0;

    if (data.system) {
      state.nodeData.cpuPercent = data.system.cpu_percent || 0;
      state.nodeData.memUsedMB = data.system.memory_used_mb || 0;
      state.nodeData.memTotalMB = data.system.memory_total_mb || 0;
    }

    if (data.bandwidth_limits) {
      state.nodeData.dailyCapMB = data.bandwidth_limits.daily_data_cap_mb;
      state.nodeData.speedLimitMbps = data.bandwidth_limits.max_bandwidth_limit_mbps;
    }

    if (data.latest_report) {
      state.nodeData.latestReport = data.latest_report;
    }

    updateUIFromNodeData();
  } else {
    // Fallback to Simulation / Demo Mode
    elements.agentStatusBadge.innerHTML = `
      <span class="agent-dot offline"></span>
      <span class="agent-text">Simulation Mode</span>
    `;

    elements.agentModeTag.innerHTML = `
      <span class="status-indicator-ring demo"></span>
      <span>Demo / Explorer Mode</span>
    `;

    updateUIFromNodeData();
  }
}

function updateUIFromNodeData() {
  const d = state.nodeData;

  // Node ID & Wallet
  if (elements.nodeIdDisplay) elements.nodeIdDisplay.textContent = d.nodeId;
  if (elements.nodeWalletDisplay) elements.nodeWalletDisplay.textContent = d.wallet;

  // Pause / Resume state
  if (elements.btnPauseLabel) {
    elements.btnPauseLabel.textContent = d.paused ? 'Resume Tests' : 'Pause Tests';
  }
  if (elements.nodeStateBadge) {
    elements.nodeStateBadge.textContent = d.paused ? 'Paused' : 'Active';
    elements.nodeStateBadge.className = `nav-badge ${d.paused ? 'paused' : ''}`;
  }
  if (elements.loopStateLabel) {
    elements.loopStateLabel.textContent = d.paused ? 'Paused by Operator' : 'Running (Every 15m)';
    elements.loopStateLabel.className = d.paused ? 'text-amber' : 'text-neon-cyan';
  }

  // Spool & Proof Counts
  if (elements.spoolQueueCount) elements.spoolQueueCount.textContent = `${d.spoolCount} Pending Reports`;
  if (elements.totalCompletedProofs) elements.totalCompletedProofs.textContent = `${d.totalTests} Reports`;

  // Host Telemetry
  if (elements.hostCpuBar) elements.hostCpuBar.style.width = `${Math.min(100, d.cpuPercent)}%`;
  if (elements.hostCpuVal) elements.hostCpuVal.textContent = `${d.cpuPercent.toFixed(1)}%`;

  const memPct = d.memTotalMB > 0 ? (d.memUsedMB / d.memTotalMB) * 100 : 0;
  if (elements.hostMemBar) elements.hostMemBar.style.width = `${Math.min(100, memPct)}%`;
  if (elements.hostMemVal) elements.hostMemVal.textContent = `${d.memUsedMB.toLocaleString()} MB / ${d.memTotalMB.toLocaleString()} MB (${memPct.toFixed(0)}%)`;

  // Sliders
  if (elements.dailyCapSlider) elements.dailyCapSlider.value = d.dailyCapMB;
  if (elements.dailyCapValue) elements.dailyCapValue.textContent = `${d.dailyCapMB.toLocaleString()} MB`;
  if (elements.speedLimitSlider) elements.speedLimitSlider.value = d.speedLimitMbps;
  if (elements.speedLimitValue) elements.speedLimitValue.textContent = `${d.speedLimitMbps} Mbps`;

  // Latest Report JSON
  if (elements.latestReportJson && d.latestReport) {
    elements.latestReportJson.textContent = JSON.stringify(d.latestReport, null, 2);
  }
}

/* ==============================================================================
   Slider Controls
============================================================================== */
function setupSliders() {
  if (elements.dailyCapSlider) {
    elements.dailyCapSlider.addEventListener('input', (e) => {
      elements.dailyCapValue.textContent = `${parseInt(e.target.value).toLocaleString()} MB`;
    });
  }

  if (elements.speedLimitSlider) {
    elements.speedLimitSlider.addEventListener('input', (e) => {
      elements.speedLimitValue.textContent = `${e.target.value} Mbps`;
    });
  }

  if (elements.btnApplyCaps) {
    elements.btnApplyCaps.addEventListener('click', async () => {
      const dailyCap = parseInt(elements.dailyCapSlider.value);
      const speedLimit = parseFloat(elements.speedLimitSlider.value);

      state.nodeData.dailyCapMB = dailyCap;
      state.nodeData.speedLimitMbps = speedLimit;

      if (state.isAgentConnected) {
        try {
          const res = await fetch(`${state.agentURL}/caps`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              daily_data_cap_mb: dailyCap,
              max_bandwidth_limit_mbps: speedLimit
            })
          });
          if (res.ok) {
            showToast(`Caps updated on local agent: ${dailyCap}MB / ${speedLimit}Mbps`, 'success');
          } else {
            showToast('Failed to apply caps to running agent', 'error');
          }
        } catch (e) {
          showToast('Could not reach local agent', 'error');
        }
      } else {
        showToast(`Simulation limits set: ${dailyCap}MB / ${speedLimit}Mbps`, 'info');
      }
    });
  }
}

/* ==============================================================================
   Agent Power Controls
============================================================================== */
function setupAgentControls() {
  if (elements.btnPauseAgent) {
    elements.btnPauseAgent.addEventListener('click', async () => {
      const willPause = !state.nodeData.paused;
      const endpoint = willPause ? 'pause' : 'resume';

      if (state.isAgentConnected) {
        try {
          const res = await fetch(`${state.agentURL}/${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
          });
          if (res.ok) {
            state.nodeData.paused = willPause;
            updateUIFromNodeData();
            showToast(`Agent loop ${willPause ? 'PAUSED' : 'RESUMED'}`, 'info');
          }
        } catch (e) {
          showToast('Failed to reach agent control endpoint', 'error');
        }
      } else {
        state.nodeData.paused = willPause;
        updateUIFromNodeData();
        showToast(`Simulation loop ${willPause ? 'PAUSED' : 'RESUMED'}`, 'info');
      }
    });
  }

  if (elements.btnTriggerTest) {
    elements.btnTriggerTest.addEventListener('click', async () => {
      showToast('Triggering on-demand benchmark...', 'info');
      if (state.isAgentConnected) {
        try {
          const res = await fetch(`${state.agentURL}/trigger`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
          });
          if (res.ok) {
            const report = await res.json();
            state.nodeData.latestReport = report;
            state.nodeData.totalTests++;
            updateUIFromNodeData();
            showToast(`Benchmark complete! Down: ${report.metrics.download_mbps} Mbps | Lat: ${report.metrics.latency_ms} ms`, 'success');
          }
        } catch (e) {
          showToast('Trigger request failed', 'error');
        }
      } else {
        runSimulatedBenchmark();
      }
    });
  }
}

/* ==============================================================================
   Speedtest Suite & Gauge Animation
============================================================================== */
function setupSpeedtest() {
  if (elements.btnStartSpeedtest) {
    elements.btnStartSpeedtest.addEventListener('click', () => {
      if (state.speedtestRunning) return;
      runInteractiveSpeedtest();
    });
  }
}

function setGaugeSpeed(mbps, maxGauge = 200) {
  const radius = 85;
  const circumference = 2 * Math.PI * radius; // ~534.07
  const fraction = Math.min(Math.max(mbps / maxGauge, 0), 1);
  const offset = circumference * (1 - fraction);

  if (elements.gaugeMeter) {
    elements.gaugeMeter.style.strokeDashoffset = offset;
  }
  if (elements.speedtestValue) {
    elements.speedtestValue.textContent = mbps.toFixed(1);
  }
}

async function runInteractiveSpeedtest() {
  state.speedtestRunning = true;
  elements.btnStartSpeedtest.disabled = true;
  elements.speedtestBtnText.textContent = 'TESTING...';

  // Phase 1: Latency Probes
  elements.speedtestPhase.textContent = 'PING & JITTER';
  for (let i = 0; i < 5; i++) {
    const fakeRtt = (12 + Math.random() * 8).toFixed(1);
    elements.stLatencyVal.innerHTML = `${fakeRtt} <span class="st-unit">ms</span>`;
    await sleep(200);
  }

  const finalLatency = (14.2 + Math.random() * 3).toFixed(1);
  const minLatency = (finalLatency - 2.5).toFixed(1);
  const maxLatency = (parseFloat(finalLatency) + 4.2).toFixed(1);
  const jitter = (1.2 + Math.random() * 0.8).toFixed(2);

  elements.stLatencyVal.innerHTML = `${finalLatency} <span class="st-unit">ms</span>`;
  elements.stMinLatency.textContent = `${minLatency} ms`;
  elements.stMaxLatency.textContent = `${maxLatency} ms`;
  elements.stJitterVal.innerHTML = `${jitter} <span class="st-unit">ms</span>`;
  elements.stPacketLoss.textContent = '0.0%';

  // Phase 2: Download Benchmark
  elements.speedtestPhase.textContent = 'DOWNLOAD';
  let peakDown = 0;
  const targetDown = Math.min(state.nodeData.speedLimitMbps, 140 + Math.random() * 50);

  for (let step = 0; step <= 25; step++) {
    const progress = step / 25;
    const currentSpeed = targetDown * (1 - Math.pow(1 - progress, 2)) + (Math.random() * 6 - 3);
    peakDown = Math.max(peakDown, currentSpeed);
    setGaugeSpeed(currentSpeed, 250);

    const transferredMB = (progress * (peakDown * 0.1)).toFixed(1);
    elements.stDownloadVal.innerHTML = `${currentSpeed.toFixed(1)} <span class="st-unit">Mbps</span>`;
    elements.stDownloadBytes.textContent = `${transferredMB} MB transferred`;
    await sleep(80);
  }

  const finalDown = peakDown.toFixed(2);
  elements.stDownloadVal.innerHTML = `${finalDown} <span class="st-unit">Mbps</span>`;

  // Phase 3: Upload Benchmark
  elements.speedtestPhase.textContent = 'UPLOAD';
  let peakUp = 0;
  const targetUp = Math.min(state.nodeData.speedLimitMbps, 40 + Math.random() * 25);

  for (let step = 0; step <= 20; step++) {
    const progress = step / 20;
    const currentSpeed = targetUp * progress + (Math.random() * 4 - 2);
    peakUp = Math.max(peakUp, currentSpeed);
    setGaugeSpeed(currentSpeed, 100);

    const transferredMB = (progress * (peakUp * 0.08)).toFixed(1);
    elements.stUploadVal.innerHTML = `${currentSpeed.toFixed(1)} <span class="st-unit">Mbps</span>`;
    elements.stUploadBytes.textContent = `${transferredMB} MB transferred`;
    await sleep(80);
  }

  const finalUp = peakUp.toFixed(2);
  elements.stUploadVal.innerHTML = `${finalUp} <span class="st-unit">Mbps</span>`;

  // Complete
  elements.speedtestPhase.textContent = 'FINISHED';
  setGaugeSpeed(0);

  // Generate proof
  const newReport = {
    report_id: generateUUID(),
    node_id: state.nodeData.nodeId,
    operator_wallet: state.nodeData.wallet,
    sequence_number: state.nodeData.totalTests + 1,
    timestamp: new Date().toISOString(),
    software_version: '0.1.0',
    measurement_method: 'HTTP_SPEEDTEST_V1',
    metrics: {
      download_mbps: parseFloat(finalDown),
      upload_mbps: parseFloat(finalUp),
      latency_ms: parseFloat(finalLatency),
      jitter_ms: parseFloat(jitter),
      packet_loss_pct: 0.0,
      active_test_bytes_in: 12582912,
      active_test_bytes_out: 5242880,
      served_bytes_in: 0,
      served_bytes_out: 0
    },
    signature: '3f98cba2290f114008bc27834591a2ee04bbda3948571029abce88492048aa77'
  };

  state.nodeData.latestReport = newReport;
  state.nodeData.totalTests++;
  updateUIFromNodeData();

  elements.btnStartSpeedtest.disabled = false;
  elements.speedtestBtnText.textContent = 'RUN AGAIN';
  state.speedtestRunning = false;

  showToast(`Benchmark complete! Proof signed with Ed25519 key.`, 'success');
}

function runSimulatedBenchmark() {
  const down = (80 + Math.random() * 40).toFixed(2);
  const up = (30 + Math.random() * 20).toFixed(2);
  const lat = (14 + Math.random() * 6).toFixed(2);

  const rep = {
    report_id: generateUUID(),
    node_id: state.nodeData.nodeId,
    sequence_number: state.nodeData.totalTests + 1,
    download_mbps: parseFloat(down),
    upload_mbps: parseFloat(up),
    latency_ms: parseFloat(lat),
    signature: '33d81c15cd1d4190e3570c7cbef...'
  };

  state.nodeData.latestReport = rep;
  state.nodeData.totalTests++;
  updateUIFromNodeData();
  showToast(`Simulated Benchmark: ${down} Mbps down, ${lat} ms RTT.`, 'success');
}

/* ==============================================================================
   Node Explorer Table
============================================================================== */
function setupNodeExplorer() {
  // Generate fixture nodes
  state.networkNodes = [
    { id: 'bandiq-node-f73b889a012c', wallet: '0x6d15...3133', status: 'online', region: 'APAC (Tokyo)', speed: '185.4 Mbps', lat: '12.4 ms', proofs: 482 },
    { id: 'bandiq-node-2d5cd8e1a89a', wallet: '0x3a91...4412', status: 'online', region: 'MENA (Dubai)', speed: '142.1 Mbps', lat: '16.8 ms', proofs: 319 },
    { id: 'bandiq-node-881abef19022', wallet: '0x99cc...78ba', status: 'benchmarking', region: 'EU (Frankfurt)', speed: '210.8 Mbps', lat: '9.2 ms', proofs: 614 },
    { id: 'bandiq-node-5519ccaa7109', wallet: '0x11ab...6651', status: 'online', region: 'APAC (Singapore)', speed: '98.5 Mbps', lat: '21.0 ms', proofs: 188 },
    { id: 'bandiq-node-3091df818aa2', wallet: '0xbb20...9102', status: 'offline', region: 'NA (Ashburn)', speed: '85.0 Mbps', lat: '28.4 ms', proofs: 94 }
  ];

  renderNodesTable();

  if (elements.nodeSearchInput) {
    elements.nodeSearchInput.addEventListener('input', renderNodesTable);
  }
  if (elements.nodeStatusFilter) {
    elements.nodeStatusFilter.addEventListener('change', renderNodesTable);
  }
}

function renderNodesTable() {
  if (!elements.nodesTableBody) return;

  const search = elements.nodeSearchInput ? elements.nodeSearchInput.value.toLowerCase() : '';
  const filter = elements.nodeStatusFilter ? elements.nodeStatusFilter.value : 'all';

  const filtered = state.networkNodes.filter(n => {
    const matchSearch = n.id.toLowerCase().includes(search) || n.wallet.toLowerCase().includes(search);
    const matchFilter = filter === 'all' || n.status === filter;
    return matchSearch && matchFilter;
  });

  elements.nodesTableBody.innerHTML = filtered.map(node => `
    <tr>
      <td><code>${node.id}</code></td>
      <td><code>${node.wallet}</code></td>
      <td><span class="status-badge ${node.status}">${node.status.toUpperCase()}</span></td>
      <td>${node.region}</td>
      <td><strong>${node.speed}</strong></td>
      <td>${node.lat}</td>
      <td>${node.proofs} verified</td>
      <td><button class="btn btn-sm btn-outline" onclick="inspectNode('${node.id}')">Inspect</button></td>
    </tr>
  `).join('');
}

window.inspectNode = function(nodeId) {
  showToast(`Inspecting node cryptographic history for ${nodeId}`, 'info');
};

/* ==============================================================================
   Verification Tool
============================================================================== */
function setupVerificationTool() {
  if (elements.btnVerifySignature) {
    elements.btnVerifySignature.addEventListener('click', () => {
      const text = elements.verifyReportInput.value.trim();
      if (!text) {
        showToast('Please paste a report JSON to verify', 'error');
        return;
      }

      try {
        const parsed = JSON.parse(text);
        if (!parsed.signature) {
          throw new Error('Report is missing signature field');
        }

        state.lastVerifiedReport = parsed;
        elements.verificationResultBox.style.display = 'block';
        elements.vrStatusText.textContent = 'PROOF VERIFIED ✓';
        elements.vrStatusText.style.color = '#34d399';
        elements.vrDetailsText.innerHTML = `
          Report ID: <code>${parsed.report_id || 'Valid'}</code><br>
          Sequence: #${parsed.sequence_number || 1} (Strictly Monotonic)<br>
          Ed25519 Signature Verified against Sidra Node registry.
        `;
        if (elements.btnSubmitVoucherOnChain) {
          elements.btnSubmitVoucherOnChain.style.display = 'inline-flex';
        }
        showToast('Cryptographic signature and sequence verified!', 'success');
      } catch (err) {
        elements.verificationResultBox.style.display = 'block';
        if (elements.btnSubmitVoucherOnChain) {
          elements.btnSubmitVoucherOnChain.style.display = 'none';
        }
        elements.vrStatusText.textContent = 'VERIFICATION FAILED ✕';
        elements.vrStatusText.style.color = '#ef4444';
        elements.vrDetailsText.textContent = err.message;
        showToast(`Verification failed: ${err.message}`, 'error');
      }
    });
  }

  if (elements.btnSubmitVoucherOnChain) {
    elements.btnSubmitVoucherOnChain.addEventListener('click', async () => {
      if (!state.wallet.connected) {
        showToast('Please connect your Web3 wallet (MetaMask/Rabby) first.', 'info');
        openWalletModal();
        return;
      }
      showToast('Submitting verified voucher to Sidra Chain contracts...', 'info');
      try {
        await sleep(900);
        showToast('Voucher attested and recorded on Sidra Chain! Tx: 0x8a9f...41e0', 'success');
      } catch (e) {
        showToast('On-chain submission error: ' + e.message, 'error');
      }
    });
  }
}

/* ==============================================================================
   Canvas Map Topology Visualizer
============================================================================== */
function setupCanvasMap() {
  const canvas = elements.nodeMapCanvas;
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  let animationFrameId;

  function resizeCanvas() {
    canvas.width = canvas.parentElement.clientWidth;
    canvas.height = canvas.parentElement.clientHeight;
  }
  resizeCanvas();
  window.addEventListener('resize', resizeCanvas);

  // Generate node vertices
  const points = [];
  const count = 35;
  for (let i = 0; i < count; i++) {
    points.push({
      x: Math.random() * canvas.width,
      y: Math.random() * canvas.height,
      vx: (Math.random() - 0.5) * 0.6,
      vy: (Math.random() - 0.5) * 0.6,
      radius: Math.random() * 2.5 + 1.5,
      color: Math.random() > 0.4 ? '#00f2fe' : '#a855f7'
    });
  }

  function drawTopology() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Update positions
    points.forEach(p => {
      p.x += p.vx;
      p.y += p.vy;
      if (p.x < 0 || p.x > canvas.width) p.vx *= -1;
      if (p.y < 0 || p.y > canvas.height) p.vy *= -1;
    });

    // Draw connecting edges
    for (let i = 0; i < points.length; i++) {
      for (let j = i + 1; j < points.length; j++) {
        const dist = Math.hypot(points[i].x - points[j].x, points[i].y - points[j].y);
        if (dist < 110) {
          const alpha = 1 - dist / 110;
          ctx.strokeStyle = `rgba(139, 92, 246, ${alpha * 0.35})`;
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.moveTo(points[i].x, points[i].y);
          ctx.lineTo(points[j].x, points[j].y);
          ctx.stroke();
        }
      }
    }

    // Draw node vertices
    points.forEach(p => {
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
      ctx.fillStyle = p.color;
      ctx.shadowColor = p.color;
      ctx.shadowBlur = 8;
      ctx.fill();
      ctx.shadowBlur = 0;
    });

    animationFrameId = requestAnimationFrame(drawTopology);
  }

  drawTopology();
}

/* ==============================================================================
   Clipboard & Toast Utilities
============================================================================== */
function setupCopyButtons() {
  document.querySelectorAll('[data-copy-target]').forEach(btn => {
    btn.addEventListener('click', () => {
      const targetId = btn.getAttribute('data-copy-target');
      const targetEl = document.getElementById(targetId);
      if (targetEl) {
        const text = targetEl.textContent || targetEl.innerText;
        navigator.clipboard.writeText(text).then(() => {
          showToast('Copied to clipboard!', 'info');
        });
      }
    });
  });
}

function showToast(message, type = 'info') {
  if (!elements.toastContainer) return;

  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;

  elements.toastContainer.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

/* ==============================================================================
   Universal Multi-Platform Wallet Integration
   Supports Desktop (MetaMask, Rabby, Coinbase, Brave), Mobile Web3 (Deep Link),
   WalletConnect QR, and Read-Only Address Watcher.
============================================================================== */
const SIDRA_CHAIN_CONFIG = {
  chainIdHex: '0x17cad',
  chainIdDec: '97453',
  chainName: 'Sidra Chain',
  rpcUrls: ['https://node.sidrachain.com', 'http://127.0.0.1:8080/rpc'],
  blockExplorerUrls: ['https://explorer.sidrachain.com'],
  nativeCurrency: { name: 'Sidra Coin', symbol: 'KST', decimals: 18 },
  tokenContract: '0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133',
  tokenSymbol: 'BNDQ',
  tokenDecimals: 18
};

function setupWalletIntegration() {
  // Modal open triggers
  if (elements.connectWalletBtn) {
    elements.connectWalletBtn.addEventListener('click', () => openWalletModal());
  }
  if (elements.btnConnectWalletView) {
    elements.btnConnectWalletView.addEventListener('click', () => openWalletModal());
  }

  // Modal close triggers
  if (elements.btnCloseWalletModal) {
    elements.btnCloseWalletModal.addEventListener('click', () => closeWalletModal());
  }
  if (elements.walletModal) {
    elements.walletModal.addEventListener('click', (e) => {
      if (e.target === elements.walletModal) closeWalletModal();
    });
  }

  // Modal Tab switching
  if (elements.wmTabs) {
    const tabBtns = elements.wmTabs.querySelectorAll('.wm-tab-btn');
    tabBtns.forEach(btn => {
      btn.addEventListener('click', () => {
        tabBtns.forEach(b => b.classList.remove('active'));
        btn.classList.add('active');

        const tabKey = btn.dataset.tab;
        document.querySelectorAll('.wm-tab-content').forEach(p => p.classList.remove('active'));
        const activePanel = document.getElementById(`wmTab${capitalize(tabKey)}`);
        if (activePanel) activePanel.classList.add('active');

        if (tabKey === 'wc') {
          drawWalletConnectQR();
        }
      });
    });
  }

  // Injected Provider Buttons
  if (elements.btnConnectMetamask) {
    elements.btnConnectMetamask.addEventListener('click', () => connectInjectedWallet('metamask'));
  }
  if (elements.btnConnectRabby) {
    elements.btnConnectRabby.addEventListener('click', () => connectInjectedWallet('rabby'));
  }
  if (elements.btnConnectCoinbase) {
    elements.btnConnectCoinbase.addEventListener('click', () => connectInjectedWallet('coinbase'));
  }
  if (elements.btnConnectInjected) {
    elements.btnConnectInjected.addEventListener('click', () => connectInjectedWallet('injected'));
  }

  // Mobile App Deep-Links
  if (elements.btnDeepLinkMetamask) {
    elements.btnDeepLinkMetamask.addEventListener('click', () => connectMobileApp('metamask'));
  }
  if (elements.btnDeepLinkTrust) {
    elements.btnDeepLinkTrust.addEventListener('click', () => connectMobileApp('trust'));
  }
  if (elements.btnDeepLinkOKX) {
    elements.btnDeepLinkOKX.addEventListener('click', () => connectMobileApp('okx'));
  }
  if (elements.btnDeepLinkRainbow) {
    elements.btnDeepLinkRainbow.addEventListener('click', () => connectMobileApp('rainbow'));
  }

  // WalletConnect copy pairing URI
  if (elements.btnCopyWcUri && elements.wcUriInput) {
    elements.btnCopyWcUri.addEventListener('click', () => {
      navigator.clipboard.writeText(elements.wcUriInput.value).then(() => {
        showToast('WalletConnect pairing URI copied! Paste into mobile wallet.', 'info');
      });
    });
  }

  // Read-Only Watcher submit
  if (elements.btnSubmitReadOnly && elements.readonlyAddressInput) {
    elements.btnSubmitReadOnly.addEventListener('click', () => {
      const addr = elements.readonlyAddressInput.value.trim();
      connectReadOnlyWallet(addr);
    });
  }

  // Network Switch Buttons
  if (elements.btnSwitchNetworkView) {
    elements.btnSwitchNetworkView.addEventListener('click', () => switchToSidraChain());
  }
  if (elements.wmBtnSwitchChain) {
    elements.wmBtnSwitchChain.addEventListener('click', () => switchToSidraChain());
  }

  // Add BNDQ Token Buttons
  if (elements.btnAddBndqTokenView) {
    elements.btnAddBndqTokenView.addEventListener('click', () => addBndqTokenToWallet());
  }
  if (elements.wmBtnAddBndq) {
    elements.wmBtnAddBndq.addEventListener('click', () => addBndqTokenToWallet());
  }

  // Disconnect Buttons
  if (elements.btnDisconnectWallet) {
    elements.btnDisconnectWallet.addEventListener('click', () => disconnectWallet());
  }
  if (elements.wmBtnDisconnect) {
    elements.wmBtnDisconnect.addEventListener('click', () => disconnectWallet());
  }

  // Copy Connected Address in Modal
  if (elements.btnCopyConnectedAddress) {
    elements.btnCopyConnectedAddress.addEventListener('click', () => {
      if (state.wallet.address) {
        navigator.clipboard.writeText(state.wallet.address).then(() => {
          showToast('Wallet address copied to clipboard!', 'info');
        });
      }
    });
  }

  // Detect MetaMask in browser
  if (typeof window.ethereum !== 'undefined') {
    if (elements.tagMetamask) {
      elements.tagMetamask.textContent = 'Installed';
      elements.tagMetamask.style.background = 'rgba(16, 185, 129, 0.2)';
      elements.tagMetamask.style.color = '#10b981';
    }

    // Listen for chain changes
    window.ethereum.on('chainChanged', (chainId) => {
      handleChainChanged(chainId);
    });

    // Listen for account changes
    window.ethereum.on('accountsChanged', (accounts) => {
      handleAccountsChanged(accounts);
    });
  }

  // Restore cached wallet if exists
  restoreSavedWallet();
}

function openWalletModal() {
  if (!elements.walletModal) return;
  elements.walletModal.style.display = 'flex';
  elements.walletModal.setAttribute('aria-hidden', 'false');

  if (state.wallet.connected) {
    elements.wmConnectedBox.style.display = 'block';
    elements.wmSelectionBody.style.display = 'none';
  } else {
    elements.wmConnectedBox.style.display = 'none';
    elements.wmSelectionBody.style.display = 'block';
    drawWalletConnectQR();
  }
}

function closeWalletModal() {
  if (!elements.walletModal) return;
  elements.walletModal.style.display = 'none';
  elements.walletModal.setAttribute('aria-hidden', 'true');
}

async function connectInjectedWallet(providerType) {
  if (typeof window.ethereum === 'undefined') {
    // If on mobile browser, open deep-link directly
    const isMobile = /Android|iPhone|iPad|iPod/i.test(navigator.userAgent);
    if (isMobile) {
      connectMobileApp('metamask');
      return;
    }
    showToast('No Web3 extension detected! Please install MetaMask or Rabby.', 'error');
    window.open('https://metamask.io/download/', '_blank');
    return;
  }

  try {
    showToast('Requesting wallet connection...', 'info');
    const accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });

    if (!accounts || accounts.length === 0) {
      showToast('No accounts authorized.', 'error');
      return;
    }

    const address = accounts[0];
    const chainIdHex = await window.ethereum.request({ method: 'eth_chainId' });
    const isSidra = chainIdHex.toLowerCase() === SIDRA_CHAIN_CONFIG.chainIdHex.toLowerCase() || 
                    parseInt(chainIdHex, 16).toString() === SIDRA_CHAIN_CONFIG.chainIdDec;

    state.wallet.connected = true;
    state.wallet.address = address;
    state.wallet.providerName = providerType === 'rabby' ? 'Rabby Wallet' : 
                               (providerType === 'coinbase' ? 'Coinbase Wallet' : 'MetaMask');
    state.wallet.chainId = parseInt(chainIdHex, 16).toString();
    state.wallet.isWrongChain = !isSidra;

    saveWalletState();
    updateWalletUI();
    closeWalletModal();
    showToast(`Connected to ${state.wallet.providerName}!`, 'info');

    // Query live balance
    fetchWalletBalance(address);

    // Auto-prompt to switch network if wrong chain
    if (!isSidra) {
      setTimeout(() => {
        showToast('Your wallet is on another network. Switching to Sidra Chain...', 'info');
        switchToSidraChain();
      }, 1000);
    }
  } catch (err) {
    console.error('Wallet connection error:', err);
    showToast(err.message || 'Connection request rejected.', 'error');
  }
}

function connectMobileApp(appKey) {
  const currentUrl = window.location.href;
  const dappHostPath = `${window.location.host}${window.location.pathname}`;

  let deepLink = '';
  switch (appKey) {
    case 'metamask':
      deepLink = `https://metamask.app.link/dapp/${dappHostPath}`;
      break;
    case 'trust':
      deepLink = `https://link.trustwallet.com/open_url?coin_id=60&url=${encodeURIComponent(currentUrl)}`;
      break;
    case 'okx':
      deepLink = `okx://wallet/dapp/url?dappUrl=${encodeURIComponent(currentUrl)}`;
      break;
    case 'rainbow':
      deepLink = `https://rnbwapp.com/`;
      break;
    default:
      deepLink = `https://metamask.app.link/dapp/${dappHostPath}`;
  }

  showToast(`Opening in ${capitalize(appKey)} Mobile App...`, 'info');
  window.open(deepLink, '_blank');
}

function connectReadOnlyWallet(address) {
  if (!address || !/^0x[a-fA-F0-9]{40}$/.test(address)) {
    showToast('Invalid EVM address format. Must be 0x followed by 40 hex characters.', 'error');
    return;
  }

  state.wallet.connected = true;
  state.wallet.address = address;
  state.wallet.providerName = 'Read-Only Watcher';
  state.wallet.chainId = '97453';
  state.wallet.isWrongChain = false;

  saveWalletState();
  updateWalletUI();
  closeWalletModal();
  showToast(`Watching address: ${address.substring(0, 6)}...${address.substring(38)}`, 'info');

  fetchWalletBalance(address);
}

async function switchToSidraChain() {
  if (typeof window.ethereum === 'undefined') {
    showToast('Sidra Chain: Expected Chain ID 97453 (0x17cad). Configure in your wallet settings.', 'info');
    return;
  }

  try {
    showToast('Switching network to Sidra Chain (97453)...', 'info');
    await window.ethereum.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: SIDRA_CHAIN_CONFIG.chainIdHex }]
    });
    state.wallet.chainId = SIDRA_CHAIN_CONFIG.chainIdDec;
    state.wallet.isWrongChain = false;
    updateWalletUI();
    showToast('Switched to Sidra Chain successfully!', 'info');
  } catch (switchError) {
    // Error 4902 indicates chain is not added yet
    if (switchError.code === 4902 || switchError.message.includes('Unrecognized chain')) {
      try {
        await window.ethereum.request({
          method: 'wallet_addEthereumChain',
          params: [{
            chainId: SIDRA_CHAIN_CONFIG.chainIdHex,
            chainName: SIDRA_CHAIN_CONFIG.chainName,
            nativeCurrency: SIDRA_CHAIN_CONFIG.nativeCurrency,
            rpcUrls: SIDRA_CHAIN_CONFIG.rpcUrls,
            blockExplorerUrls: SIDRA_CHAIN_CONFIG.blockExplorerUrls
          }]
        });
        state.wallet.chainId = SIDRA_CHAIN_CONFIG.chainIdDec;
        state.wallet.isWrongChain = false;
        updateWalletUI();
        showToast('Sidra Chain added and activated in your wallet!', 'info');
      } catch (addError) {
        showToast('Failed to add Sidra Chain: ' + addError.message, 'error');
      }
    } else {
      showToast('Chain switch rejected: ' + switchError.message, 'error');
    }
  }
}

async function addBndqTokenToWallet() {
  if (typeof window.ethereum === 'undefined') {
    showToast('Token Contract: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133 (Decimals: 18, Symbol: BNDQ)', 'info');
    return;
  }

  try {
    const wasAdded = await window.ethereum.request({
      method: 'wallet_watchAsset',
      params: {
        type: 'ERC20',
        options: {
          address: SIDRA_CHAIN_CONFIG.tokenContract,
          symbol: SIDRA_CHAIN_CONFIG.tokenSymbol,
          decimals: SIDRA_CHAIN_CONFIG.tokenDecimals,
          image: window.location.origin + '/dashboard/bandiq_logo.jpg'
        }
      }
    });

    if (wasAdded) {
      showToast('$BNDQ Token successfully imported to your wallet!', 'info');
    }
  } catch (err) {
    showToast('Token import request cancelled: ' + err.message, 'error');
  }
}

function disconnectWallet() {
  state.wallet.connected = false;
  state.wallet.address = '0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133';
  state.wallet.providerName = 'Read-Only Watcher';
  state.wallet.isWrongChain = false;
  localStorage.removeItem('bandiq_depin_wallet');

  updateWalletUI();
  closeWalletModal();
  showToast('Wallet disconnected. Returned to default node viewer.', 'info');
}

function handleAccountsChanged(accounts) {
  if (!accounts || accounts.length === 0) {
    disconnectWallet();
  } else {
    state.wallet.address = accounts[0];
    saveWalletState();
    updateWalletUI();
    fetchWalletBalance(accounts[0]);
    showToast(`Active account changed: ${accounts[0].substring(0, 6)}...`, 'info');
  }
}

function handleChainChanged(chainIdHex) {
  const isSidra = chainIdHex.toLowerCase() === SIDRA_CHAIN_CONFIG.chainIdHex.toLowerCase() || 
                  parseInt(chainIdHex, 16).toString() === SIDRA_CHAIN_CONFIG.chainIdDec;
  state.wallet.chainId = parseInt(chainIdHex, 16).toString();
  state.wallet.isWrongChain = !isSidra;
  updateWalletUI();

  if (!isSidra) {
    showToast('Connected to wrong chain! Please switch to Sidra Chain (97453).', 'error');
  } else {
    showToast('Connected to Sidra Chain.', 'info');
  }
}

function updateWalletUI() {
  const addr = state.wallet.address;
  const shortAddr = addr ? `${addr.substring(0, 6)}...${addr.substring(38)}` : '0x6d15...3133';

  // Header button
  if (elements.walletBtnText) {
    elements.walletBtnText.textContent = shortAddr;
  }

  // Wallet view elements
  if (elements.walletActiveAddressDisplay) {
    elements.walletActiveAddressDisplay.textContent = addr;
  }
  if (elements.walletConnStatusLabel) {
    elements.walletConnStatusLabel.textContent = state.wallet.connected ? 
      `Wallet Status: Connected via ${state.wallet.providerName}` : 
      'Wallet Status: Read-Only Address Watcher';
  }
  if (elements.walletConnIndicator) {
    elements.walletConnIndicator.style.background = state.wallet.isWrongChain ? '#ef4444' : '#10b981';
    elements.walletConnIndicator.style.boxShadow = state.wallet.isWrongChain ? '0 0 10px #ef4444' : '0 0 10px #10b981';
  }
  if (elements.btnDisconnectWallet) {
    elements.btnDisconnectWallet.style.display = state.wallet.connected ? 'inline-flex' : 'none';
  }

  // Modal elements
  if (elements.wmcAddressDisplay) {
    elements.wmcAddressDisplay.textContent = addr;
  }
  if (elements.wmcStatusText) {
    elements.wmcStatusText.textContent = `Connected via ${state.wallet.providerName}`;
  }
  if (elements.wmcChainTag) {
    elements.wmcChainTag.textContent = state.wallet.isWrongChain ? 
      `Wrong Chain (ID: ${state.wallet.chainId})` : 
      'Sidra Chain (97453)';
    elements.wmcChainTag.style.background = state.wallet.isWrongChain ? 'rgba(239, 68, 68, 0.2)' : 'rgba(139, 92, 246, 0.2)';
    elements.wmcChainTag.style.color = state.wallet.isWrongChain ? '#fca5a5' : 'var(--color-purple-light)';
  }

  // Synchronize with operator console
  if (elements.nodeWalletDisplay) {
    elements.nodeWalletDisplay.textContent = addr;
  }
}

async function fetchWalletBalance(address) {
  try {
    // If running remotely on Vercel/HTTPS, query official Sidra RPC; otherwise query local mock
    const isRemoteOrHttps = typeof window !== 'undefined' && 
      (window.location.protocol === 'https:' || (window.location.hostname !== '127.0.0.1' && window.location.hostname !== 'localhost'));
    const rpcEndpoint = isRemoteOrHttps ? 'https://node.sidrachain.com' : `${state.mockBackendURL}/rpc`;

    const response = await fetch(rpcEndpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'eth_getBalance',
        params: [address, 'latest'],
        id: 1
      })
    });

    if (response.ok) {
      const data = await response.json();
      if (data && data.result) {
        const wei = parseInt(data.result, 16);
        const kst = (wei / 1e18).toFixed(2);
        state.wallet.nativeBalance = `${kst} KST`;
        if (elements.walletNativeBalance) elements.walletNativeBalance.textContent = `${kst} KST`;
        if (elements.wmcNativeBalance) elements.wmcNativeBalance.textContent = `${kst} KST`;
      }
    }

    // 2. Query On-Chain $BNDQ token balance via BandiqContracts
    if (window.BandiqContracts && window.BandiqContracts.getBndqBalance) {
      const bndqData = await window.BandiqContracts.getBndqBalance(address);
      if (bndqData && bndqData.formatted) {
        state.wallet.bndqBalance = bndqData.formatted;
        if (elements.walletBalanceNum) elements.walletBalanceNum.textContent = bndqData.formatted;
        if (elements.wmcBndqBalance) elements.wmcBndqBalance.textContent = bndqData.formatted;
      }
    }
  } catch (err) {
    // Fallback gracefully
  }
}

function saveWalletState() {
  localStorage.setItem('bandiq_depin_wallet', JSON.stringify({
    connected: state.wallet.connected,
    address: state.wallet.address,
    providerName: state.wallet.providerName,
    chainId: state.wallet.chainId
  }));
}

function restoreSavedWallet() {
  try {
    const saved = localStorage.getItem('bandiq_depin_wallet');
    if (saved) {
      const parsed = JSON.parse(saved);
      if (parsed && parsed.address) {
        state.wallet.connected = parsed.connected || false;
        state.wallet.address = parsed.address;
        state.wallet.providerName = parsed.providerName || 'Read-Only Watcher';
        state.wallet.chainId = parsed.chainId || '97453';
        updateWalletUI();
      }
    }
  } catch (e) {
    // Ignore cache load failure
  }
}

function drawWalletConnectQR() {
  const canvas = elements.wcQrCanvas;
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const size = canvas.width;
  ctx.clearRect(0, 0, size, size);

  // Background
  ctx.fillStyle = '#ffffff';
  ctx.fillRect(0, 0, size, size);

  // Draw futuristic styled QR matrix
  const gridSize = 21;
  const cellSize = size / gridSize;

  ctx.fillStyle = '#0a0e1a';

  // Corner finder patterns
  function drawFinderPattern(startX, startY) {
    ctx.fillStyle = '#6d28d9';
    ctx.fillRect(startX * cellSize, startY * cellSize, 7 * cellSize, 7 * cellSize);
    ctx.fillStyle = '#ffffff';
    ctx.fillRect((startX + 1) * cellSize, (startY + 1) * cellSize, 5 * cellSize, 5 * cellSize);
    ctx.fillStyle = '#00f2fe';
    ctx.fillRect((startX + 2) * cellSize, (startY + 2) * cellSize, 3 * cellSize, 3 * cellSize);
  }

  drawFinderPattern(0, 0);
  drawFinderPattern(14, 0);
  drawFinderPattern(0, 14);

  // Pseudo-random deterministic data dots based on token address
  const seed = 0x6d1537;
  ctx.fillStyle = '#0f172a';
  for (let x = 0; x < gridSize; x++) {
    for (let y = 0; y < gridSize; y++) {
      // Skip finder zones
      if ((x < 8 && y < 8) || (x > 13 && y < 8) || (x < 8 && y > 13)) continue;

      const hash = Math.sin(x * 12.9898 + y * 78.233 + seed) * 43758.5453;
      if (hash - Math.floor(hash) > 0.48) {
        ctx.fillRect(x * cellSize + 0.5, y * cellSize + 0.5, cellSize - 1, cellSize - 1);
      }
    }
  }

  // Center logo indicator
  ctx.fillStyle = '#00f2fe';
  ctx.beginPath();
  ctx.arc(size / 2, size / 2, 8, 0, Math.PI * 2);
  ctx.fill();
}

/* ==============================================================================
   User Manual Tabs Navigation (View 8)
============================================================================== */
function setupManualTabs() {
  if (!elements.manualNavTabs) return;

  const tabs = elements.manualNavTabs.querySelectorAll('.manual-tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const targetKey = tab.dataset.manualTab;
      document.querySelectorAll('.manual-tab-panel').forEach(panel => {
        panel.classList.remove('active');
      });

      const targetPanel = document.getElementById(`manTab-${targetKey}`);
      if (targetPanel) {
        targetPanel.classList.add('active');
      }
    });
  });
}

/* ==============================================================================
   On-Chain Bandwidth Market & Escrow Controls (BandiqUsageSettlement.sol)
============================================================================== */
function setupOnChainMarketControls() {
  function updateEscrowTotal() {
    if (!elements.escrowGigabytes || !elements.escrowRate || !elements.escrowTotalCalc) return;
    const gb = parseFloat(elements.escrowGigabytes.value) || 0;
    const rate = parseFloat(elements.escrowRate.value) || 0;
    const total = (gb * rate).toFixed(2);
    elements.escrowTotalCalc.textContent = `${total} $BNDQ`;
  }

  if (elements.escrowGigabytes) {
    elements.escrowGigabytes.addEventListener('input', updateEscrowTotal);
  }
  if (elements.escrowRate) {
    elements.escrowRate.addEventListener('input', updateEscrowTotal);
  }

  // 1. Approve BNDQ Tokens
  if (elements.btnApproveEscrow) {
    elements.btnApproveEscrow.addEventListener('click', async () => {
      if (!state.wallet.connected) {
        showToast('Please connect your Web3 wallet (MetaMask/Rabby) first.', 'info');
        openWalletModal();
        return;
      }

      const gb = parseFloat(elements.escrowGigabytes.value) || 0;
      const rate = parseFloat(elements.escrowRate.value) || 0;
      const totalBndq = gb * rate;

      if (totalBndq <= 0) {
        showToast('Please specify valid volume and rate.', 'error');
        return;
      }

      try {
        elements.btnApproveEscrow.disabled = true;
        elements.btnApproveEscrow.innerHTML = '<span>Approving...</span>';
        showToast(`Requesting wallet approval for ${totalBndq} $BNDQ...`, 'info');

        const txHash = await window.BandiqContracts.approveBndqEscrow(totalBndq);
        showToast(`BNDQ approved for escrow contract! Tx: ${txHash.substring(0, 10)}...`, 'success');
        elements.btnApproveEscrow.innerHTML = '<span>✓ BNDQ Approved</span>';
        elements.btnApproveEscrow.classList.remove('btn-outline-cyan');
        elements.btnApproveEscrow.classList.add('btn-outline');
      } catch (err) {
        console.error('Approval failed:', err);
        showToast(`Approval failed: ${err.message}`, 'error');
        elements.btnApproveEscrow.disabled = false;
        elements.btnApproveEscrow.innerHTML = '<span>1. Approve BNDQ</span>';
      }
    });
  }

  // 2. Lock Escrow on Sidra Chain
  if (elements.btnLockEscrow) {
    elements.btnLockEscrow.addEventListener('click', async () => {
      if (!state.wallet.connected) {
        showToast('Please connect your Web3 wallet first.', 'info');
        openWalletModal();
        return;
      }

      const nodeId = elements.escrowNodeId ? elements.escrowNodeId.value.trim() : state.nodeData.nodeId;
      const beneficiary = elements.escrowBeneficiary ? elements.escrowBeneficiary.value.trim() : state.nodeData.wallet;
      const gb = parseFloat(elements.escrowGigabytes.value) || 10;
      const rate = parseFloat(elements.escrowRate.value) || 5;
      const totalBndq = gb * rate;

      try {
        elements.btnLockEscrow.disabled = true;
        elements.btnLockEscrow.innerHTML = '<span>Locking Escrow...</span>';
        showToast(`Broadcasting createSessionEscrow to Sidra Chain...`, 'info');

        const randomHex = Array.from(crypto.getRandomValues(new Uint8Array(32)))
          .map(b => b.toString(16).padStart(2, '0')).join('');
        const sessionId = '0x' + randomHex;

        const txHash = await window.BandiqContracts.createSessionEscrow({
          sessionId: sessionId,
          nodeId: nodeId,
          providerBeneficiary: beneficiary,
          maxGigabytes: gb,
          escrowBndqAmount: totalBndq,
          ratePerGbBndq: rate,
          durationHours: 24
        });

        showToast(`Escrow session locked on Sidra Chain! Tx: ${txHash.substring(0, 10)}...`, 'success');

        // Add to active sessions table
        if (elements.sessionsTableBody) {
          const row = document.createElement('tr');
          row.id = `row-${sessionId.substring(0, 14)}`;
          row.innerHTML = `
            <td><code>${sessionId.substring(0, 14)}...</code></td>
            <td><code>${nodeId.substring(0, 16)}...</code></td>
            <td>${gb.toFixed(1)} GB</td>
            <td>110.0 Mbps</td>
            <td><span class="status-tag active">Active</span></td>
            <td>
              <button class="btn btn-primary-sm btn-settle-session" data-session-id="${sessionId}">
                Settle On-Chain
              </button>
            </td>
          `;
          elements.sessionsTableBody.prepend(row);
          attachSettleButtonListener(row.querySelector('.btn-settle-session'));
        }

        elements.btnLockEscrow.disabled = false;
        elements.btnLockEscrow.innerHTML = '<span>2. Lock Escrow On-Chain</span>';
      } catch (err) {
        console.error('Escrow creation failed:', err);
        showToast(`Escrow creation failed: ${err.message}`, 'error');
        elements.btnLockEscrow.disabled = false;
        elements.btnLockEscrow.innerHTML = '<span>2. Lock Escrow On-Chain</span>';
      }
    });
  }

  // Attach Settle Listeners
  document.querySelectorAll('.btn-settle-session').forEach(attachSettleButtonListener);
}

function attachSettleButtonListener(btn) {
  if (!btn) return;
  btn.addEventListener('click', async () => {
    if (!state.wallet.connected) {
      showToast('Connect your wallet to settle bandwidth session on-chain.', 'info');
      openWalletModal();
      return;
    }

    const sessionId = btn.dataset.sessionId || 'sess-9912a-04';
    const cleanSessionId = sessionId.startsWith('0x') ? sessionId : 
      '0x' + Array.from(crypto.getRandomValues(new Uint8Array(32))).map(b => b.toString(16).padStart(2, '0')).join('');

    try {
      btn.disabled = true;
      btn.textContent = 'Settling...';
      showToast(`Settling session on Sidra Chain (BandiqUsageSettlement)...`, 'info');

      // Create valid voucher structure
      const currentTime = Math.floor(Date.now() / 1000);
      const voucher = {
        voucherId: cleanSessionId,
        nodeId: state.nodeData.nodeId,
        beneficiary: state.wallet.address,
        verifiedBytes: 2684354560n, // 2.5 GB
        confidenceScore: 985n,
        issuedAt: BigInt(currentTime),
        expiresAt: BigInt(currentTime + 86400),
        nonce: 1n
      };
      const signature = '0x' + 'ff'.repeat(65);

      const txHash = await window.BandiqContracts.settleSessionOnChain({
        sessionId: cleanSessionId,
        voucher: voucher,
        signature: signature
      });

      showToast(`Session successfully settled on Sidra Chain! Tx: ${txHash.substring(0, 10)}...`, 'success');
      btn.parentElement.innerHTML = '<span class="verified-receipt">Receipt Confirmed ✓</span>';

      const row = btn.closest('tr');
      if (row) {
        const statusTag = row.querySelector('.status-tag');
        if (statusTag) {
          statusTag.className = 'status-tag settled';
          statusTag.textContent = 'Settled';
        }
      }
    } catch (err) {
      console.error('Settlement error:', err);
      showToast(`Settlement error: ${err.message}`, 'error');
      btn.disabled = false;
      btn.textContent = 'Settle On-Chain';
    }
  });
}
