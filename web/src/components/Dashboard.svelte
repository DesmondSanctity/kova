<script>
  import { onMount } from 'svelte';
  import { fade, fly, scale } from 'svelte/transition';
  import Toaster from './Toaster.svelte';
  import Counter from './Counter.svelte';
  import Sparkline from './Sparkline.svelte';
  import AreaChart from './AreaChart.svelte';
  import { toast } from '../lib/toast.svelte.js';

  let loading = $state(true);
  let user = $state(null);
  let workspace = $state(null);
  let keys = $state([]);
  let usage = $state({ total: 0, last30: 0, byKind: {}, daily: [] });
  let links = $state([]);
  let section = $state('overview');
  let userMenu = $state(false);
  let newMenu = $state(false);

  const icons = {
    overview:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></svg>',
    keys:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="7.5" cy="15.5" r="4.5"/><path d="M10.5 12.5 20 3M16 7l3 3M14 9l2 2" stroke-linecap="round"/></svg>',
    links:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>',
    usage:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/></svg>',
    billing:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="2.5" y="5" width="19" height="14" rx="2.5"/><path d="M2.5 9.5h19" stroke-linecap="round"/></svg>',
    settings:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="12" cy="12" r="3"/><path d="M19 12a7 7 0 0 0-.1-1l2-1.6-2-3.4-2.3 1a7 7 0 0 0-1.7-1l-.3-2.5H10.4l-.3 2.5a7 7 0 0 0-1.7 1l-2.3-1-2 3.4 2 1.6a7 7 0 0 0 0 2l-2 1.6 2 3.4 2.3-1a7 7 0 0 0 1.7 1l.3 2.5h3.2l.3-2.5a7 7 0 0 0 1.7-1l2.3 1 2-3.4-2-1.6a7 7 0 0 0 .1-1Z"/></svg>',
    activity:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>',
    repayments:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M3 10h18M7 15h4M6 4h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z"/></svg>',
    doc:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M14 3v5h5"/><path d="M18 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8l6 6v10a2 2 0 0 1-2 2Z"/></svg>',
    back:
      '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6"/></svg>',
    check:
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>',
    copy:
      '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg>',
  };

  const groups = [
    { label: '', items: [['overview', 'Overview'], ['keys', 'API keys'], ['links', 'Links']] },
    { label: 'Insights', items: [['usage', 'Usage'], ['repayments', 'Repayments'], ['billing', 'Billing'], ['activity', 'Activity']] },
    { label: 'Workspace', items: [['settings', 'Settings']] },
  ];

  const titles = { overview: 'Overview', keys: 'API keys', links: 'Links', usage: 'Usage', billing: 'Billing', settings: 'Settings', activity: 'Activity', repayments: 'Repayments' };

  // fetch wrapper: any authenticated call that 401s means the session died.
  async function api(path, opts) {
    const r = await fetch(path, opts);
    if (r.status === 401) {
      toast.warning('Your session expired. Taking you to sign in…');
      setTimeout(() => (location.href = '/login'), 1500);
      throw new Error('unauthorized');
    }
    return r;
  }

  onMount(async () => {
    const h = location.hash.replace('#', '');
    if (titles[h]) section = h;
    try {
      const r = await fetch('/api/me');
      if (r.status === 401) { location.href = '/login'; return; }
      const me = await r.json();
      if (me.needsOnboarding) { location.href = '/onboarding'; return; }
      user = me.user;
      workspace = me.workspace;
      keys = me.keys || [];
      usage = me.usage || usage;
      seedSettings();
      loading = false;
      loadLinks();
      loadAudit();
    } catch {
      loading = false;
    }

    const onKey = (e) => { if (e.key === 'Escape') { modal = null; created = null; detail = null; userMenu = false; newMenu = false; } };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  function go(id) {
    section = id;
    newMenu = false;
    if (id === 'activity') loadAudit();
    history.replaceState(null, '', '#' + id);
  }

  const initials = (s) => (s || '?').trim().slice(0, 1).toUpperCase();
  const fmtDate = (s) => { try { return new Date(s).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }); } catch { return ''; } };

  const activeKeys = $derived(keys.filter((k) => !k.revokedAt).length);

  // Range-filtered daily series for charts.
  let rangeDays = $state(30);
  const ranges = [['7D', 7], ['30D', 30], ['90D', 90], ['1Y', 365]];

  function buildDateSeries(valueFor, n) {
    const data = [];
    const labels = [];
    for (let i = n - 1; i >= 0; i--) {
      const dt = new Date();
      dt.setDate(dt.getDate() - i);
      const key = dt.toISOString().slice(0, 10);
      data.push(valueFor(key) || 0);
      labels.push(dt.toLocaleDateString('en-GB', n > 90 ? { month: 'short', year: '2-digit' } : { day: 'numeric', month: 'short' }));
    }
    return { data, labels };
  }

  const usageByDay = $derived.by(() => {
    const m = {};
    (usage.daily || []).forEach((d) => (m[d.day] = d.count));
    return m;
  });
  const series = $derived(buildDateSeries((k) => usageByDay[k], rangeDays));
  const last7 = $derived(buildDateSeries((k) => usageByDay[k], 7).data.reduce((a, b) => a + b, 0));

  // Disbursed metrics (from links).
  const disbursedLinks = $derived(links.filter((l) => l.disbursed));
  const totalDisbursed = $derived(disbursedLinks.reduce((a, l) => a + (l.offerAmount || 0), 0) / 100);
  const disbursedByDay = $derived.by(() => {
    const m = {};
    disbursedLinks.forEach((l) => {
      const key = ((l.disbursedAt || l.createdAt) || '').slice(0, 10);
      m[key] = (m[key] || 0) + (l.offerAmount || 0) / 100;
    });
    return m;
  });
  const disbursedSeries = $derived(buildDateSeries((k) => disbursedByDay[k], rangeDays));

  // ---- modals ----
  let modal = $state(null); // 'key' | 'link'
  let created = $state(null); // { type, data }
  let busy = $state(false);
  let formName = $state('');
  let formNote = $state('');
  let formErr = $state('');

  function openModal(kind) {
    modal = kind;
    newMenu = false;
    formErr = '';
    formName = '';
    formNote = '';
  }

  async function submitKey(e) {
    e?.preventDefault();
    if (!formName.trim()) { formErr = 'Give this key a name so you can recognise it later.'; return; }
    busy = true;
    try {
      const r = await api('/api/keys', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: formName.trim() }) });
      const k = await r.json();
      if (!r.ok) { formErr = k.error || 'Could not create key.'; return; }
      keys = [...keys, k];
      modal = null;
      created = { type: 'key', data: k };
      toast.success('API key created');
    } catch (_) {
      formErr = 'Network error. Please try again.';
    } finally { busy = false; }
  }

  async function submitLink(e) {
    e?.preventDefault();
    if (!formNote.trim()) { formErr = 'Add a note (e.g. the borrower name) to identify this link.'; return; }
    busy = true;
    try {
      const r = await api('/api/links', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({
        note: formNote.trim(),
      }) });
      const l = await r.json();
      if (!r.ok) { formErr = l.error || 'Could not create link.'; return; }
      modal = null;
      created = { type: 'link', data: l };
      toast.success('Shareable link created');
      loadLinks();
    } catch (_) {
      formErr = 'Network error. Please try again.';
    } finally { busy = false; }
  }

  // ---- keys management ----
  let revealed = $state({});
  let openKey = $state(null);

  async function revokeKey(id) {
    if (!confirm('Revoke this key? Apps using it will stop working immediately.')) return;
    try {
      const r = await api('/api/keys/' + id, { method: 'DELETE' });
      if (r.ok) { keys = keys.filter((k) => k.id !== id); toast.success('Key revoked'); }
      else toast.error('Could not revoke key');
    } catch (_) {}
  }
  async function saveAllowlist(k) {
    const domains = (k._domains ?? (k.allowedDomains || []).join(', ')).split(',').map((s) => s.trim()).filter(Boolean);
    const ips = (k._ips ?? (k.allowedIps || []).join(', ')).split(',').map((s) => s.trim()).filter(Boolean);
    try {
      const r = await api('/api/keys/' + k.id + '/allowlist', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ domains, ips }) });
      if (r.ok) { k.allowedDomains = domains; k.allowedIps = ips; keys = [...keys]; toast.success('Allowlist updated'); }
      else toast.error('Could not update allowlist');
    } catch (_) {}
  }
  function copy(text, label = 'Copied') {
    navigator.clipboard?.writeText(text);
    toast.success(label);
  }

  // ---- links ----
  async function loadLinks() { try { const r = await fetch('/api/links'); if (r.ok) links = (await r.json()).links || []; } catch {} }

  // ---- audit ----
  let audit = $state([]);
  async function loadAudit() { try { const r = await fetch('/api/audit'); if (r.ok) audit = (await r.json()).events || []; } catch {} }

  // ---- table pagination (Links / Repayments / Activity) ----
  const PAGE = 10;
  let linksPage = $state(1);
  let repayPage = $state(1);
  let activityPage = $state(1);
  const linksPages = $derived(Math.max(1, Math.ceil(links.length / PAGE)));
  const repayPages = $derived(Math.max(1, Math.ceil(disbursedLinks.length / PAGE)));
  const activityPages = $derived(Math.max(1, Math.ceil(audit.length / PAGE)));
  const linksView = $derived(links.slice((Math.min(linksPage, linksPages) - 1) * PAGE, Math.min(linksPage, linksPages) * PAGE));
  const repayView = $derived(disbursedLinks.slice((Math.min(repayPage, repayPages) - 1) * PAGE, Math.min(repayPage, repayPages) * PAGE));
  const activityView = $derived(audit.slice((Math.min(activityPage, activityPages) - 1) * PAGE, Math.min(activityPage, activityPages) * PAGE));
  const auditLabel = { 'link.created': 'Link created', 'request.scored': 'Statements scored', 'offer.accepted': 'Application submitted', 'offer.declined': 'Borrower withdrew', 'offer.resent': 'Estimate resent', 'request.rejected': 'Application rejected', 'loan.otp_sent': 'Payout OTP sent', 'loan.disbursed': 'Loan disbursed', 'repayment.requested': 'Repayment link sent', 'repayment.reminder': 'Repayment reminder', 'loan.repaid': 'Loan repaid', 'key.created': 'API key created', 'key.revoked': 'API key revoked' };
  const relTime = (s) => { try { const d = (Date.now() - new Date(s)) / 1000; if (d < 60) return 'just now'; if (d < 3600) return Math.floor(d / 60) + 'm ago'; if (d < 86400) return Math.floor(d / 3600) + 'h ago'; return new Date(s).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' }); } catch { return ''; } };

  async function logout() { await fetch('/auth/logout', { method: 'POST' }); try { localStorage.setItem('kova_authed', '0'); } catch (e) {} location.href = '/login'; }

  const bandColor = (b) => (b === 'A' ? 'text-accent' : b === 'B' ? 'text-violet' : 'text-amber-400');

  // ---- settings / branding ----
  let settingsTab = $state('branding');
  const settingsTabs = [['branding', 'Branding'], ['products', 'Loan products'], ['rules', 'Lending rules'], ['payments', 'Payments'], ['account', 'Account']];
  let settingsForm = $state({ orgName: '', brandName: '', brandColor: '#8b7cff', brandTextColor: '#ffffff', supportEmail: '', minScore: '', loanProducts: [] });
  let savingSettings = $state(false);

  function seedSettings() {
    settingsForm = {
      orgName: workspace?.orgName || workspace?.name || '',
      brandName: workspace?.brandName || '',
      brandColor: workspace?.brandColor || '#8b7cff',
      brandTextColor: workspace?.brandTextColor || '#ffffff',
      supportEmail: workspace?.supportEmail || '',
      minScore: workspace?.minScore ? String(workspace.minScore) : '',
      loanProducts: (workspace?.loanProducts || []).map((p) => ({ maxAmount: p.maxAmount ? p.maxAmount / 100 : '', interestRate: p.interestRate || '', tenorDays: p.tenorDays || '' })),
    };
    seedMonnify();
  }

  // ---- payments (per-lender Monnify) ----
  let monForm = $state({ baseUrl: 'https://sandbox.monnify.com', apiKey: '', secretKey: '', contractCode: '', walletAccount: '' });
  let savingMonnify = $state(false);
  function seedMonnify() {
    monForm = {
      baseUrl: workspace?.monnifyBaseUrl || 'https://sandbox.monnify.com',
      apiKey: '',
      secretKey: '',
      contractCode: workspace?.monnifyContractCode || '',
      walletAccount: workspace?.monnifyWalletAccount || '',
    };
  }
  async function saveMonnify(e) {
    e?.preventDefault();
    savingMonnify = true;
    try {
      const r = await api('/api/workspace/monnify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(monForm),
      });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not save Monnify credentials'); return; }
      workspace = d;
      seedMonnify();
      toast.success('Monnify connected');
    } catch (_) {
      toast.error('Network error. Please try again.');
    } finally { savingMonnify = false; }
  }

  function addProduct() {
    settingsForm.loanProducts = [...(settingsForm.loanProducts || []), { maxAmount: '', interestRate: '', tenorDays: '' }];
  }
  function removeProduct(i) {
    settingsForm.loanProducts = settingsForm.loanProducts.filter((_, idx) => idx !== i);
  }

  async function saveSettings(e) {
    e?.preventDefault();
    savingSettings = true;
    try {
      const products = (settingsForm.loanProducts || [])
        .map((p) => ({ maxAmount: Math.round((parseFloat(p.maxAmount) || 0) * 100), interestRate: parseFloat(p.interestRate) || 0, tenorDays: parseInt(p.tenorDays) || 0 }))
        .filter((p) => p.maxAmount > 0);
      const r = await api('/api/workspace', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...settingsForm, minScore: Math.max(0, Math.min(100, parseInt(settingsForm.minScore) || 0)), loanProducts: products }),
      });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not save settings'); return; }
      workspace = d;
      seedSettings();
      toast.success('Workspace settings saved');
    } catch (_) {
      toast.error('Network error. Please try again.');
    } finally { savingSettings = false; }
  }

  // ---- link details ----
  let detail = $state(null); // { link, report, status, loading }
  let confirmDisburse = $state(false);
  let confirmReject = $state(false);
  let rejectReason = $state('');

  async function openDetail(link) {
    confirmDisburse = false;
    confirmReject = false;
    rejectReason = '';
    otpRequired = link.disbursementStatus === 'PENDING_AUTHORIZATION' && !link.disbursed && !link.repaid;
    otpCode = '';
    detail = { link, report: null, status: link.status, loading: true };
    try {
      const r = await fetch('/v1/requests/' + link.id);
      const d = await r.json();
      detail = { link, report: d.report || null, status: d.status, loading: false };
    } catch (_) {
      detail = { link, report: null, status: link.status, loading: false };
    }
  }

  const naira = (kobo) => '₦' + Math.round((Number(kobo) || 0) / 100).toLocaleString('en-NG');
  // For score-report values, which are already in naira (not kobo).
  const nairaN = (n) => '₦' + Math.round(Number(n) || 0).toLocaleString('en-NG');

  // Repayment-only side pane (opened from the Repayments list).
  let repayDetail = $state(null);
  let repayStatusMsg = $state('');
  function openRepay(link) {
    repayStatusMsg = '';
    repayDetail = link;
  }

  // Activity detail side pane.
  let activityDetail = $state(null);
  function openActivity(e) {
    activityDetail = e;
  }

  let disbursing = $state(false);
  let otpRequired = $state(false);
  let otpCode = $state('');
  let authorizing = $state(false);
  async function doDisburse(link) {
    disbursing = true;
    try {
      const r = await api('/api/links/' + link.id + '/disburse', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Disbursement failed'); return; }
      if (d.status === 'otp_required') {
        otpRequired = true;
        otpCode = '';
        confirmDisburse = false;
        toast.success(d.pending ? 'This payout is already awaiting authorization — enter the OTP, or resend a code.' : 'OTP sent to your Monnify email — enter it to authorize.');
        return;
      }
      toast.success('Disbursed — ref ' + d.reference);
      confirmDisburse = false;
      await loadLinks();
      detail = { ...detail, link: { ...detail.link, disbursed: true, status: 'disbursed' }, status: 'disbursed' };
    } catch (_) {
      toast.error('Network error. Please try again.');
    } finally { disbursing = false; }
  }

  async function authorizeDisburse(link) {
    if (!(otpCode || '').trim()) { toast.error('Enter the OTP from your email.'); return; }
    authorizing = true;
    try {
      const r = await api('/api/links/' + link.id + '/authorize', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ otp: otpCode.trim() }) });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Authorization failed'); return; }
      toast.success('Disbursed — ref ' + d.reference);
      otpRequired = false;
      otpCode = '';
      await loadLinks();
      detail = { ...detail, link: { ...detail.link, disbursed: true, status: 'disbursed' }, status: 'disbursed' };
    } catch (_) {
      toast.error('Network error. Please try again.');
    } finally { authorizing = false; }
  }

  async function resendOtp(link) {
    try {
      const r = await api('/api/links/' + link.id + '/resend-otp', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not resend OTP'); return; }
      toast.success('A new OTP has been sent to your Monnify email.');
    } catch (_) { toast.error('Network error. Please try again.'); }
  }

  // repayments
  const repaymentStatus = (l) => (l.repaid ? 'repaid' : l.repaymentDueAt && new Date(l.repaymentDueAt) < new Date() ? 'overdue' : 'outstanding');
  const repayColor = (st) => (st === 'repaid' ? 'text-accent' : st === 'overdue' ? 'text-red-400' : 'text-muted');
  const outstanding = $derived(disbursedLinks.filter((l) => !l.repaid).reduce((a, l) => a + (l.repaymentTotal || l.offerAmount || 0), 0));
  const repaidLinks = $derived(disbursedLinks.filter((l) => l.repaid));
  const totalRepaid = $derived(repaidLinks.reduce((a, l) => a + (l.repaymentTotal || l.offerAmount || 0), 0) / 100);
  const collectPct = $derived.by(() => {
    const book = totalRepaid + outstanding / 100;
    return book > 0 ? Math.round((totalRepaid / book) * 100) : 0;
  });
  // display status for a link — surfaces "repaid" once the loan is paid back
  const linkStatus = (l) => (l.repaid ? 'repaid' : l.status);
  const statusPill = (l) =>
    l.repaid ? 'bg-accent/15 text-accent'
    : l.status === 'declined' || l.status === 'rejected' ? 'bg-red-500/15 text-red-400'
    : 'bg-surface-2 text-muted';
  let repaying = $state(false);
  async function markRepaid(link) {
    repaying = true;
    try {
      const r = await api('/api/links/' + link.id + '/repay', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not mark repaid'); return; }
      toast.success('Marked as repaid');
      await loadLinks();
      if (detail) detail = { ...detail, link: { ...detail.link, repaid: true } };
      if (repayDetail && repayDetail.id === link.id) repayDetail = { ...repayDetail, repaid: true };
    } catch (_) { toast.error('Network error. Please try again.'); }
    finally { repaying = false; }
  }

  let requestingRepay = $state(false);
  async function requestRepayment(link) {
    requestingRepay = true;
    try {
      const r = await api('/api/links/' + link.id + '/request-repayment', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not send repayment link'); return; }
      toast.success('Repayment link emailed to the borrower');
      await loadLinks();
    } catch (_) { toast.error('Network error. Please try again.'); }
    finally { requestingRepay = false; }
  }

  let verifyingRepay = $state(false);
  async function verifyRepayment(link) {
    verifyingRepay = true;
    try {
      const r = await api('/api/links/' + link.id + '/verify-repayment', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not verify payment'); return; }
      if (d.status === 'repaid') {
        toast.success('Payment confirmed — marked as repaid');
        repayStatusMsg = 'Payment confirmed and marked as repaid.';
        await loadLinks();
        if (detail) detail = { ...detail, link: { ...detail.link, repaid: true } };
        if (repayDetail && repayDetail.id === link.id) repayDetail = { ...repayDetail, repaid: true };
      } else {
        const ps = d.paymentStatus || 'no payment found yet';
        repayStatusMsg = 'Monnify status: ' + ps;
        toast.warning('No completed payment yet (' + ps + ')');
      }
    } catch (_) { toast.error('Network error. Please try again.'); }
    finally { verifyingRepay = false; }
  }

  let resendingOffer = $state(false);
  async function resendOffer(link) {
    resendingOffer = true;
    try {
      const r = await api('/api/links/' + link.id + '/resend-offer', { method: 'POST' });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not resend the offer email'); return; }
      toast.success('Offer email resent to the borrower');
    } catch (_) { toast.error('Network error. Please try again.'); }
    finally { resendingOffer = false; }
  }

  const decisionLabel = { approved: 'Approved', counter: 'Counter-offer', declined: 'Declined' };
  const decisionColor = (d) => (d === 'approved' ? 'text-accent' : d === 'counter' ? 'text-violet' : d === 'declined' ? 'text-red-400' : 'text-faint');

  async function doReject(link) {
    try {
      const r = await api('/api/links/' + link.id + '/reject', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason: rejectReason.trim() }),
      });
      const d = await r.json();
      if (!r.ok) { toast.error(d.error || 'Could not reject request'); return; }
      toast.success('Application rejected — borrower notified');
      confirmReject = false;
      rejectReason = '';
      await loadLinks();
      detail = { ...detail, link: { ...detail.link, status: 'rejected', decision: 'declined' }, status: 'rejected' };
    } catch (_) {
      toast.error('Network error. Please try again.');
    }
  }
</script>

<style>
  .sk {
    background: linear-gradient(
      90deg,
      var(--color-surface) 25%,
      var(--color-surface-2) 50%,
      var(--color-surface) 75%
    );
    background-size: 200% 100%;
    animation: sk-shimmer 1.3s ease-in-out infinite;
  }
  @keyframes sk-shimmer {
    to {
      background-position: -200% 0;
    }
  }
</style>

<Toaster />

{#snippet pager(page, pages, onset)}
  {#if pages > 1}
    <div class="mt-4 flex items-center justify-center gap-1.5">
      <button onclick={() => onset(Math.max(1, page - 1))} disabled={page <= 1} aria-label="Previous page" class="grid h-8 w-8 place-items-center rounded-lg border border-line text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-40 disabled:hover:border-line disabled:hover:text-muted">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6"/></svg>
      </button>
      {#each Array(pages) as _, i (i)}
        <button onclick={() => onset(i + 1)} class="grid h-8 min-w-[32px] place-items-center rounded-lg border px-2 text-[13px] font-medium tabular-nums transition-colors {page === i + 1 ? 'border-violet/50 bg-violet/15 text-violet' : 'border-line text-muted hover:border-faint hover:text-fg'}">{i + 1}</button>
      {/each}
      <button onclick={() => onset(Math.min(pages, page + 1))} disabled={page >= pages} aria-label="Next page" class="grid h-8 w-8 place-items-center rounded-lg border border-line text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-40 disabled:hover:border-line disabled:hover:text-muted">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6"/></svg>
      </button>
    </div>
  {/if}
{/snippet}

{#if loading}
  <div class="min-h-screen bg-bg">
    <!-- top bar skeleton -->
    <div class="flex h-16 items-center justify-between border-b border-line px-5">
      <div class="flex items-center gap-3">
        <div class="grid h-8 w-8 place-items-center rounded-lg bg-fg text-[15px] font-extrabold text-bg">K</div>
        <div class="sk h-4 w-24 rounded"></div>
      </div>
      <div class="sk h-8 w-8 rounded-full"></div>
    </div>
    <div class="flex">
      <!-- sidebar skeleton -->
      <div class="hidden w-60 shrink-0 border-r border-line p-4 md:block">
        {#each Array(5) as _}
          <div class="sk mb-2.5 h-9 w-full rounded-lg"></div>
        {/each}
      </div>
      <!-- content skeleton -->
      <div class="flex-1 p-6">
        <div class="sk mb-2 h-7 w-48 rounded"></div>
        <div class="sk mb-8 h-4 w-64 rounded"></div>
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {#each Array(4) as _}
            <div class="rounded-2xl border border-line p-5">
              <div class="sk mb-4 h-4 w-20 rounded"></div>
              <div class="sk h-8 w-24 rounded"></div>
            </div>
          {/each}
        </div>
        <div class="sk mt-6 h-64 w-full rounded-2xl"></div>
      </div>
    </div>
  </div>
{:else}
  <!-- TOP BAR -->
  <header class="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-line bg-bg/85 px-5 backdrop-blur-xl">
    <div class="flex items-center gap-3">
      <a href="/" class="grid h-8 w-8 place-items-center rounded-lg bg-fg text-[15px] font-extrabold text-bg">K</a>
      <div class="hidden h-6 w-px bg-line sm:block"></div>
      <div class="flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[15px]">
        <span class="grid h-6 w-6 place-items-center rounded-md bg-violet/20 text-[12px] font-semibold text-violet">{initials(workspace?.orgName || workspace?.name)}</span>
        <span class="font-medium text-fg">{workspace?.orgName || workspace?.name}</span>
      </div>
    </div>

    <div class="flex items-center gap-2.5">
      <button onclick={() => openModal('link')} class="flex items-center gap-2 rounded-lg bg-fg px-3.5 py-2 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        New link
      </button>

      <div class="relative">
        <button onclick={() => (userMenu = !userMenu)} class="grid h-9 w-9 place-items-center rounded-full bg-surface-2 text-[14px] font-semibold text-fg ring-1 ring-line transition-shadow hover:ring-faint">{initials(user?.name)}</button>
        {#if userMenu}
          <div transition:fly={{ y: -6, duration: 160 }} class="absolute right-0 mt-2 w-60 rounded-xl border border-line bg-surface p-1.5 shadow-2xl">
            <div class="px-3 py-2.5">
              <div class="text-[14px] font-medium text-fg">{user?.name}</div>
              <div class="truncate text-[13px] text-faint">{user?.email}</div>
            </div>
            <div class="my-1 h-px bg-line"></div>
            <button onclick={() => { go('settings'); userMenu = false; }} class="block w-full rounded-lg px-3 py-2.5 text-left text-[14px] text-muted hover:bg-surface-2 hover:text-fg">Settings</button>
            <button onclick={logout} class="block w-full rounded-lg px-3 py-2.5 text-left text-[14px] text-muted hover:bg-surface-2 hover:text-fg">Sign out</button>
          </div>
        {/if}
      </div>
    </div>
  </header>

  <div class="md:grid md:grid-cols-[248px_1fr]">
    <!-- SIDEBAR -->
    <aside class="sticky top-16 hidden h-[calc(100vh-4rem)] flex-col border-r border-line bg-surface/30 p-3.5 md:flex">
      <nav class="flex-1 space-y-6">
        {#each groups as grp (grp.label)}
          <div>
            {#if grp.label}
              <div class="px-3 pb-2 font-mono text-[11px] uppercase tracking-wider text-faint">{grp.label}</div>
            {/if}
            <div class="space-y-1">
              {#each grp.items as [id, label] (id)}
                <button onclick={() => go(id)} class="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-[15px] transition-colors {section === id ? 'bg-surface-2 font-medium text-fg' : 'text-muted hover:bg-surface-2/60 hover:text-fg'}">
                  <span class={section === id ? 'text-violet' : 'text-faint'}>{@html icons[id]}</span>
                  {label}
                  {#if id === 'keys'}<span class="ml-auto rounded-full border border-violet/30 bg-violet/10 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-violet">Soon</span>{/if}
                </button>
              {/each}
            </div>
          </div>
        {/each}
      </nav>
      <div class="space-y-1 border-t border-line pt-3.5">
        <a href="/docs" class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-[15px] text-muted transition-colors hover:text-fg"><span class="text-faint">{@html icons.doc}</span> Documentation</a>
        <a href="/" class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-[15px] text-muted transition-colors hover:text-fg"><span class="text-faint">{@html icons.back}</span> Back to site</a>
      </div>
    </aside>

    <!-- MAIN -->
    <main class="min-w-0">
      <div class="flex gap-2 overflow-x-auto border-b border-line px-4 py-3 md:hidden">
        {#each groups.flatMap((g) => g.items) as [id, label] (id)}
          <button onclick={() => go(id)} class="whitespace-nowrap rounded-lg px-3 py-2 text-[14px] {section === id ? 'bg-surface-2 text-fg' : 'text-muted'}">{label}</button>
        {/each}
      </div>

      <div class="mx-auto max-w-[1100px] px-6 py-10 lg:px-10">
        {#if section === 'overview'}
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h1 class="text-[32px] font-light tracking-tight">Welcome back, {(user?.name || '').split(' ')[0] || 'there'}</h1>
              <p class="mt-1.5 text-[16px] text-muted">Here is what is happening across {workspace?.orgName || 'your workspace'}.</p>
            </div>
            <span class="rounded-lg bg-violet/15 px-3 py-1.5 text-[13px] font-medium capitalize text-violet">{workspace?.plan} plan</span>
          </div>

          <!-- STAT TILES -->
          <div class="mt-8 grid gap-4 lg:grid-cols-3">
            <!-- Featured: lending portfolio -->
            <div class="rounded-2xl border border-line bg-surface p-6 lg:col-span-2 lg:row-span-2 lg:p-7">
              <div class="flex items-start justify-between">
                <div>
                  <span class="text-[14px] text-muted">Total disbursed</span>
                  <div class="mt-2 text-[46px] font-light leading-none tracking-tight">₦<Counter value={totalDisbursed} /></div>
                  <div class="mt-2 text-[13.5px] text-faint">{disbursedLinks.length} loan{disbursedLinks.length === 1 ? '' : 's'} paid out</div>
                </div>
                <span class="grid h-11 w-11 place-items-center rounded-xl bg-accent/12 text-accent">{@html icons.billing}</span>
              </div>

              <div class="mt-8">
                <div class="flex items-end justify-between text-[13px]">
                  <span class="text-muted">Collections</span>
                  <span class="tabular-nums text-faint">{collectPct}% repaid</span>
                </div>
                <div class="mt-2 flex h-2.5 gap-0.5 overflow-hidden rounded-full bg-surface-2">
                  <div class="h-full rounded-l-full bg-accent transition-all duration-500" style="width:{collectPct}%"></div>
                  <div class="h-full flex-1 bg-violet/60 transition-all duration-500"></div>
                </div>
                <div class="mt-4 grid grid-cols-2 gap-3">
                  <button onclick={() => go('repayments')} class="group rounded-xl border border-line bg-bg/40 p-4 text-left transition-colors hover:border-faint">
                    <div class="flex items-center gap-1.5 text-[12.5px] text-faint"><span class="h-2 w-2 rounded-full bg-accent"></span> Repaid</div>
                    <div class="mt-1.5 text-[22px] font-light tracking-tight">₦<Counter value={totalRepaid} /></div>
                    <div class="mt-0.5 text-[12px] text-faint">{repaidLinks.length} of {disbursedLinks.length} loan{disbursedLinks.length === 1 ? '' : 's'}</div>
                  </button>
                  <button onclick={() => go('repayments')} class="group rounded-xl border border-line bg-bg/40 p-4 text-left transition-colors hover:border-faint">
                    <div class="flex items-center gap-1.5 text-[12.5px] text-faint"><span class="h-2 w-2 rounded-full bg-violet/60"></span> Outstanding</div>
                    <div class="mt-1.5 text-[22px] font-light tracking-tight">₦<Counter value={Math.round(outstanding / 100)} /></div>
                    <div class="mt-0.5 text-[12px] text-faint">{disbursedLinks.length - repaidLinks.length} loan{disbursedLinks.length - repaidLinks.length === 1 ? '' : 's'} owing</div>
                  </button>
                </div>
              </div>
            </div>

            <!-- Checks all time -->
            <div class="rounded-2xl border border-line bg-surface p-6">
              <div class="flex items-center justify-between">
                <span class="text-[14px] text-muted">Checks all time</span>
                <span class="text-violet">{@html icons.usage}</span>
              </div>
              <div class="mt-3 text-[34px] font-light leading-none tracking-tight"><Counter value={usage.total} /></div>
              <div class="mt-4"><Sparkline data={series.data} /></div>
            </div>

            <!-- Last 30 days -->
            <div class="rounded-2xl border border-line bg-surface p-6">
              <div class="flex items-center justify-between">
                <span class="text-[14px] text-muted">Last 30 days</span>
                <span class="text-accent">{@html icons.overview}</span>
              </div>
              <div class="mt-3 text-[34px] font-light leading-none tracking-tight"><Counter value={usage.last30} /></div>
              <div class="mt-4 text-[13.5px] text-faint">
                {#if last7 > 0}<span class="text-accent">+{last7}</span> in the last 7 days{:else}No checks this week yet{/if}
              </div>
            </div>
          </div>

          <!-- CHARTS -->
          <div class="mt-6 flex items-center justify-end">
            <div class="inline-flex rounded-lg border border-line bg-surface p-0.5">
              {#each ranges as [label, days] (days)}
                <button onclick={() => (rangeDays = days)} class="rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors {rangeDays === days ? 'bg-surface-2 text-fg' : 'text-muted hover:text-fg'}">{label}</button>
              {/each}
            </div>
          </div>
          <div class="mt-3 grid gap-4 lg:grid-cols-2">
            <div class="rounded-2xl border border-line bg-surface p-6 lg:p-7">
              <div class="flex items-center justify-between">
                <div>
                  <h2 class="text-[18px] font-medium text-fg">Checks over time</h2>
                  <p class="mt-1 text-[14px] text-muted">Scoring volume per day.</p>
                </div>
                <div class="flex items-center gap-2 text-[13px] text-muted"><span class="h-2.5 w-2.5 rounded-full bg-violet"></span> Checks</div>
              </div>
              <div class="mt-5"><AreaChart data={series.data} labels={series.labels} emptyText="No checks in this period" /></div>
            </div>

            <div class="rounded-2xl border border-line bg-surface p-6 lg:p-7">
              <div class="flex items-center justify-between">
                <div>
                  <h2 class="text-[18px] font-medium text-fg">Disbursed over time</h2>
                  <p class="mt-1 text-[14px] text-muted">Amount paid out per day.</p>
                </div>
                <div class="flex items-center gap-2 text-[13px] text-muted"><span class="h-2.5 w-2.5 rounded-full bg-accent"></span> Naira</div>
              </div>
              <div class="mt-5"><AreaChart data={disbursedSeries.data} labels={disbursedSeries.labels} emptyText="Nothing disbursed in this period" /></div>
            </div>
          </div>

          <!-- RECENT LINKS -->
          <div class="mt-6 flex items-center justify-between">
            <h2 class="text-[18px] font-medium text-fg">Recent links</h2>
            <button onclick={() => openModal('link')} class="rounded-lg border border-line px-3.5 py-2 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg">New link</button>
          </div>
          <div class="mt-3 overflow-hidden rounded-2xl border border-line">
            <table class="w-full text-left text-[15px]">
              <thead class="border-b border-line bg-surface/60 font-mono text-[12px] uppercase tracking-wider text-faint">
                <tr><th class="px-5 py-3 font-normal">Note</th><th class="px-5 py-3 font-normal">Status</th><th class="px-5 py-3 font-normal">Score</th><th class="px-5 py-3 font-normal">Created</th></tr>
              </thead>
              <tbody>
                {#each links.slice(0, 5) as l (l.id)}
                  <tr class="border-b border-line/60 last:border-0 transition-colors hover:bg-surface/50">
                    <td class="px-5 py-3.5"><button onclick={() => openDetail(l)} class="font-medium text-fg hover:text-violet hover:underline">{l.note || 'Untitled request'}</button></td>
                    <td class="px-5 py-3.5"><span class="rounded-md px-2.5 py-1 text-[13px] capitalize {statusPill(l)}">{linkStatus(l)}</span></td>
                    <td class="px-5 py-3.5">{#if l.score != null}<span class="font-medium text-fg">{l.score}</span> <span class={bandColor(l.band)}>· {l.band}</span>{:else}<span class="text-faint">—</span>{/if}</td>
                    <td class="px-5 py-3.5 text-faint">{fmtDate(l.createdAt)}</td>
                  </tr>
                {:else}
                  <tr><td colspan="4" class="px-5 py-10 text-center text-[15px] text-faint">No links yet. Create one to share with a borrower.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>

        {:else if section === 'keys'}
          <div class="flex flex-wrap items-center gap-3">
            <h1 class="text-[30px] font-light tracking-tight">API keys</h1>
            <span class="rounded-full border border-violet/30 bg-violet/10 px-3 py-1 text-[12px] font-medium uppercase tracking-wide text-violet">Coming soon</span>
          </div>
          <p class="mt-1.5 text-[16px] text-muted">Programmatic access with publishable and secret keys — for the SDK widget and REST API.</p>

          <div class="mt-8 overflow-hidden rounded-2xl border border-line bg-surface">
            <div class="grid items-center gap-8 p-8 sm:grid-cols-[1.1fr_1fr] lg:p-10">
              <div>
                <div class="inline-flex items-center gap-2 rounded-lg bg-violet/10 px-3 py-1.5 text-[13px] font-medium text-violet">{@html icons.keys} In development</div>
                <h2 class="mt-4 text-[22px] font-medium tracking-tight text-fg">Score from your own app</h2>
                <p class="mt-2.5 max-w-[46ch] text-[15px] leading-relaxed text-muted">
                  Soon you'll create per-workspace keys to embed the drop-in upload widget and call the scoring API directly — with domain and IP allowlisting you control.
                </p>
                <ul class="mt-5 space-y-2.5 text-[14.5px] text-muted">
                  <li class="flex items-center gap-2.5"><span class="text-accent">{@html icons.check}</span> Publishable keys for the browser SDK</li>
                  <li class="flex items-center gap-2.5"><span class="text-accent">{@html icons.check}</span> Secret keys for server-side scoring</li>
                  <li class="flex items-center gap-2.5"><span class="text-accent">{@html icons.check}</span> Domain &amp; IP allowlists per key</li>
                </ul>
                <p class="mt-6 text-[14px] text-faint">Available today: share a link and run the full lend loop from your dashboard.</p>
                <button onclick={() => go('links')} class="mt-3 rounded-lg bg-fg px-4 py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90">Create a link instead →</button>
              </div>
              <div class="rounded-xl border border-line bg-bg/40 p-5 opacity-90">
                <div class="font-mono text-[11px] uppercase tracking-wider text-faint">Preview</div>
                <div class="mt-3 space-y-2.5 font-mono text-[13px]">
                  <div class="flex items-center gap-2.5"><span class="w-[86px] text-faint">publishable</span><code class="flex-1 truncate rounded-lg bg-surface px-3 py-2 text-muted">pk_live_••••••••••••••••</code></div>
                  <div class="flex items-center gap-2.5"><span class="w-[86px] text-faint">secret</span><code class="flex-1 truncate rounded-lg bg-surface px-3 py-2 text-muted">sk_live_••••••••••••••••</code></div>
                </div>
                <div class="mt-4 rounded-lg border border-dashed border-line px-3 py-2.5 text-center text-[12.5px] text-faint">Key management opens soon</div>
              </div>
            </div>
          </div>


        {:else if section === 'links'}
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h1 class="text-[30px] font-light tracking-tight">Links</h1>
              <p class="mt-1.5 text-[16px] text-muted">Send a borrower a link, they upload a statement, you get a score.</p>
            </div>
            <button onclick={() => openModal('link')} class="flex items-center gap-2 rounded-lg bg-fg px-4 py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
              Create link
            </button>
          </div>
          <div class="mt-7 overflow-hidden rounded-2xl border border-line">
            <table class="w-full text-left text-[15px]">
              <thead class="border-b border-line bg-surface/60 font-mono text-[12px] uppercase tracking-wider text-faint">
                <tr><th class="px-5 py-3 font-normal">Note</th><th class="px-5 py-3 font-normal">Status</th><th class="px-5 py-3 font-normal">Score</th><th class="px-5 py-3 font-normal text-right">Actions</th></tr>
              </thead>
              <tbody>
                {#each linksView as l (l.id)}
                  <tr class="border-b border-line/60 last:border-0 transition-colors hover:bg-surface/50">
                    <td class="px-5 py-3.5"><button onclick={() => openDetail(l)} class="font-medium text-fg hover:text-violet hover:underline">{l.note || 'Untitled request'}</button></td>
                    <td class="px-5 py-3.5"><span class="rounded-md px-2.5 py-1 text-[13px] capitalize {statusPill(l)}">{linkStatus(l)}</span></td>
                    <td class="px-5 py-3.5">{#if l.score != null}<span class="font-medium text-fg">{l.score}</span> <span class={bandColor(l.band)}>· {l.band}</span>{:else}<span class="text-faint">—</span>{/if}</td>
                    <td class="px-5 py-3.5 text-right"><button onclick={() => openDetail(l)} class="text-[14px] font-medium text-violet hover:underline">Details</button><button onclick={() => copy(l.borrowerUrl, 'Link copied to clipboard')} class="ml-4 text-[14px] text-muted hover:text-fg">Copy link</button></td>
                  </tr>
                {:else}
                  <tr><td colspan="4" class="px-5 py-12 text-center text-[15px] text-faint">No links yet. Create one above and share it with a borrower.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
          {@render pager(Math.min(linksPage, linksPages), linksPages, (n) => (linksPage = n))}

        {:else if section === 'usage'}
          <h1 class="text-[30px] font-light tracking-tight">Usage</h1>
          <p class="mt-1.5 text-[16px] text-muted">Scoring activity across your workspace.</p>
          <div class="mt-7 grid gap-4 sm:grid-cols-2">
            <div class="rounded-2xl border border-line bg-surface p-6"><div class="text-[14px] text-muted">Total checks</div><div class="mt-3 text-[40px] font-light"><Counter value={usage.total} /></div></div>
            <div class="rounded-2xl border border-line bg-surface p-6"><div class="text-[14px] text-muted">Last 30 days</div><div class="mt-3 text-[40px] font-light"><Counter value={usage.last30} /></div></div>
          </div>
          <div class="mt-4 rounded-2xl border border-line bg-surface p-6 lg:p-7">
            <h2 class="text-[18px] font-medium text-fg">Daily volume</h2>
            <div class="mt-5"><AreaChart data={series.data} labels={series.labels} /></div>
          </div>
          <div class="mt-4 rounded-2xl border border-line bg-surface p-6">
            <div class="text-[14px] text-muted">By type</div>
            <div class="mt-4 space-y-3">
              {#each Object.entries(usage.byKind || {}) as [kind, n] (kind)}
                <div class="flex items-center justify-between text-[15px]"><span class="capitalize text-muted">{kind}</span><span class="font-medium text-fg">{n}</span></div>
              {:else}
                <div class="text-[15px] text-faint">No usage yet.</div>
              {/each}
            </div>
          </div>

        {:else if section === 'billing'}
          <h1 class="text-[30px] font-light tracking-tight">Billing</h1>
          <p class="mt-1.5 text-[16px] text-muted">You are on the pilot plan.</p>
          <div class="mt-7 rounded-2xl border border-line bg-surface p-7">
            <div class="flex items-baseline justify-between">
              <div><div class="text-[20px] font-medium">Pilot</div><div class="mt-1.5 text-[14px] text-muted">Free during the pilot programme.</div></div>
              <div class="text-right"><div class="text-[30px] font-light">₦50<span class="text-[15px] text-muted"> / check</span></div><div class="text-[13px] text-faint">indicative · not billed yet</div></div>
            </div>
            <div class="mt-6 border-t border-line pt-5 text-[15px] text-muted">{usage.total} checks used · <span class="text-fg">₦0 due</span></div>
          </div>

        {:else if section === 'repayments'}
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h1 class="text-[30px] font-light tracking-tight">Repayments</h1>
              <p class="mt-1.5 text-[16px] text-muted">Outstanding loans and collections.</p>
            </div>
            <div class="rounded-2xl border border-line bg-surface px-5 py-3 text-right">
              <div class="text-[13px] text-faint">Outstanding</div>
              <div class="text-[22px] font-light text-fg">{naira(outstanding)}</div>
            </div>
          </div>
          <div class="mt-7 overflow-hidden rounded-2xl border border-line">
            <table class="w-full text-left text-[15px]">
              <thead class="border-b border-line bg-surface/60 font-mono text-[12px] uppercase tracking-wider text-faint">
                <tr><th class="px-5 py-3 font-normal">Borrower</th><th class="px-5 py-3 font-normal">Principal</th><th class="px-5 py-3 font-normal">Total due</th><th class="px-5 py-3 font-normal">Due</th><th class="px-5 py-3 font-normal">Status</th></tr>
              </thead>
              <tbody>
                {#each repayView as l (l.id)}
                  {@const st = repaymentStatus(l)}
                  <tr class="cursor-pointer border-b border-line/60 transition-colors last:border-0 hover:bg-surface/50" onclick={() => openRepay(l)}>
                    <td class="px-5 py-3.5 font-medium text-fg">{l.borrowerName || l.note || 'Borrower'}</td>
                    <td class="px-5 py-3.5 text-muted">{naira(l.offerAmount)}</td>
                    <td class="px-5 py-3.5 text-fg">{naira(l.repaymentTotal || l.offerAmount)}</td>
                    <td class="px-5 py-3.5 text-faint">{l.repaymentDueAt ? fmtDate(l.repaymentDueAt) : '—'}</td>
                    <td class="px-5 py-3.5"><span class="text-[14px] font-medium capitalize {repayColor(st)}">{st}</span></td>
                  </tr>
                {:else}
                  <tr><td colspan="5" class="px-5 py-12 text-center text-[15px] text-faint">No disbursed loans yet.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
          {@render pager(Math.min(repayPage, repayPages), repayPages, (n) => (repayPage = n))}

        {:else if section === 'activity'}
          <div class="flex items-center justify-between">
            <div>
              <h1 class="text-[30px] font-light tracking-tight">Activity</h1>
              <p class="mt-1.5 text-[16px] text-muted">An audit trail of everything that happened in your workspace.</p>
            </div>
            <button onclick={loadAudit} class="rounded-lg border border-line px-3.5 py-2 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg">Refresh</button>
          </div>
          <div class="mt-7 overflow-hidden rounded-2xl border border-line">
            <table class="w-full text-left text-[15px]">
              <thead class="border-b border-line bg-surface/60 font-mono text-[12px] uppercase tracking-wider text-faint">
                <tr><th class="px-5 py-3 font-normal">Event</th><th class="px-5 py-3 font-normal">By</th><th class="px-5 py-3 font-normal">Detail</th><th class="px-5 py-3 font-normal text-right">When</th></tr>
              </thead>
              <tbody>
                {#each activityView as e (e.id)}
                  <tr class="cursor-pointer border-b border-line/60 transition-colors last:border-0 hover:bg-surface/50" onclick={() => openActivity(e)}>
                    <td class="px-5 py-3.5 font-medium text-fg">{auditLabel[e.action] || e.action}</td>
                    <td class="px-5 py-3.5"><span class="rounded-md bg-surface-2 px-2.5 py-1 text-[13px] capitalize text-muted">{e.actor}</span></td>
                    <td class="px-5 py-3.5 max-w-[280px] truncate text-muted" title={e.detail}>{e.detail || '—'}</td>
                    <td class="px-5 py-3.5 text-right text-faint">{relTime(e.createdAt)}</td>
                  </tr>
                {:else}
                  <tr><td colspan="4" class="px-5 py-12 text-center text-[15px] text-faint">No activity yet.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
          {@render pager(Math.min(activityPage, activityPages), activityPages, (n) => (activityPage = n))}

        {:else if section === 'settings'}
          <h1 class="text-[30px] font-light tracking-tight">Settings</h1>
          <p class="mt-1.5 text-[16px] text-muted">Your account, workspace, and link branding.</p>

          <!-- tabs -->
          <div class="mt-7 flex flex-wrap gap-1 border-b border-line">
            {#each settingsTabs as [id, label] (id)}
              <button onclick={() => (settingsTab = id)} class="relative -mb-px rounded-t-lg px-4 py-2.5 text-[14px] font-medium transition-colors {settingsTab === id ? 'border-b-2 border-violet text-fg' : 'text-muted hover:text-fg'}">
                {label}{#if id === 'payments' && !workspace?.monnifyConnected}<span class="ml-1.5 inline-block h-1.5 w-1.5 rounded-full bg-red-400 align-middle"></span>{/if}
              </button>
            {/each}
          </div>

          {#if settingsTab === 'account'}
            <div class="mt-6 rounded-2xl border border-line bg-surface p-6 lg:p-7">
              <div class="text-[15px] font-medium text-fg">Account</div>
              <div class="mt-4 grid gap-x-8 gap-y-5 sm:grid-cols-2 lg:grid-cols-4">
                <div><div class="text-[13px] text-faint">Name</div><div class="mt-1 text-[16px] text-fg">{user?.name}</div></div>
                <div><div class="text-[13px] text-faint">Email</div><div class="mt-1 text-[16px] lowercase text-fg">{user?.email}</div></div>
                <div><div class="text-[13px] text-faint">Use case</div><div class="mt-1 text-[16px] capitalize text-fg">{workspace?.useCase || '—'}</div></div>
                <div><div class="text-[13px] text-faint">Plan</div><div class="mt-1 text-[16px] capitalize text-fg">{workspace?.plan || '—'}</div></div>
              </div>
            </div>
            <button onclick={logout} class="mt-5 rounded-lg border border-line px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-red-400">Sign out</button>

          {:else if settingsTab === 'payments'}
            <form onsubmit={saveMonnify} class="mt-6 rounded-2xl border border-line bg-surface p-6 lg:p-7">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <div class="text-[15px] font-medium text-fg">Payments · Monnify</div>
                  <p class="mt-1 text-[13.5px] text-muted">Kova disburses and collects on your own Monnify account. Secrets are encrypted at rest.</p>
                </div>
                {#if workspace?.monnifyConnected}
                  <span class="shrink-0 rounded-full bg-accent/15 px-3 py-1 text-[12.5px] font-medium text-accent">Connected</span>
                {:else}
                  <span class="shrink-0 rounded-full bg-red-500/15 px-3 py-1 text-[12.5px] font-medium text-red-400">Not connected</span>
                {/if}
              </div>
              <div class="mt-4 rounded-lg border border-line bg-bg/40 px-3.5 py-3 text-[13px] leading-relaxed text-muted">
                Where to find these — log in to <a href="https://app.monnify.com" target="_blank" rel="noopener" class="text-violet underline underline-offset-2">app.monnify.com</a>:
                <ul class="mt-1.5 space-y-1">
                  <li>• <b class="text-fg">API key</b> &amp; <b class="text-fg">Secret key</b> — Settings → API Keys &amp; Webhooks</li>
                  <li>• <b class="text-fg">Contract code</b> — Settings → API Keys &amp; Webhooks (or your contract details)</li>
                  <li>• <b class="text-fg">Wallet account</b> — Disbursement → Wallet (the 10-digit source account)</li>
                </ul>
              </div>
              <div class="mt-5 grid gap-5 sm:grid-cols-2">
                <label class="block">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">Environment</span>
                  <select bind:value={monForm.baseUrl} class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg outline-none focus:border-violet">
                    <option value="https://sandbox.monnify.com">Sandbox</option>
                    <option value="https://api.monnify.com">Live</option>
                  </select>
                </label>
                <label class="block">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">Contract code</span>
                  <input bind:value={monForm.contractCode} inputmode="numeric" placeholder="1234567890" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">API key</span>
                  <input bind:value={monForm.apiKey} placeholder={workspace?.monnifyConnected ? 're-enter to update' : 'MK_PROD_XXXXXXXX'} class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">Secret key</span>
                  <input bind:value={monForm.secretKey} type="password" placeholder={workspace?.monnifyConnected ? 're-enter to update' : '••••••••••••'} class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                </label>
                <label class="block sm:col-span-2">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">Wallet account number</span>
                  <input bind:value={monForm.walletAccount} inputmode="numeric" maxlength="10" placeholder="0123456789 (10-digit source wallet)" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                </label>
              </div>
              <div class="mt-5">
                <button type="submit" disabled={savingMonnify} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{savingMonnify ? 'Verifying with Monnify…' : 'Save Monnify credentials'}</button>
              </div>
            </form>

          {:else if settingsTab === 'branding'}
            <form onsubmit={saveSettings} class="mt-6 grid items-start gap-5 lg:grid-cols-[1.6fr_1fr]">
              <div class="space-y-5">
                <div class="rounded-2xl border border-line bg-surface p-6 lg:p-7">
                  <div class="text-[15px] font-medium text-fg">Workspace &amp; branding</div>
                  <p class="mt-1 text-[13.5px] text-muted">Controls what borrowers see on your shareable link pages.</p>
                  <div class="mt-5 grid gap-5 sm:grid-cols-2">
                    <label class="block">
                      <span class="mb-2 block text-[13.5px] font-medium text-muted">Organisation name</span>
                      <input bind:value={settingsForm.orgName} placeholder="Acme Credit" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                    </label>
                    <label class="block">
                      <span class="mb-2 block text-[13.5px] font-medium text-muted">Brand name on links</span>
                      <input bind:value={settingsForm.brandName} placeholder="Defaults to organisation name" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                    </label>
                    <label class="block">
                      <span class="mb-2 block text-[13.5px] font-medium text-muted">Button &amp; accent colour</span>
                      <div class="flex items-center gap-2.5 rounded-xl border border-line bg-bg/40 px-3 py-2.5">
                        <input type="color" bind:value={settingsForm.brandColor} aria-label="Accent colour" class="h-9 w-9 shrink-0 cursor-pointer rounded-lg border border-line bg-transparent p-0.5" />
                        <input bind:value={settingsForm.brandColor} placeholder="#8b7cff" class="w-full bg-transparent font-mono text-[14px] text-fg placeholder:text-faint outline-none" />
                      </div>
                    </label>
                    <label class="block">
                      <span class="mb-2 block text-[13.5px] font-medium text-muted">Button text colour</span>
                      <div class="flex items-center gap-2.5 rounded-xl border border-line bg-bg/40 px-3 py-2.5">
                        <input type="color" bind:value={settingsForm.brandTextColor} aria-label="Button text colour" class="h-9 w-9 shrink-0 cursor-pointer rounded-lg border border-line bg-transparent p-0.5" />
                        <input bind:value={settingsForm.brandTextColor} placeholder="#ffffff" class="w-full bg-transparent font-mono text-[14px] text-fg placeholder:text-faint outline-none" />
                      </div>
                    </label>
                    <label class="block sm:col-span-2">
                      <span class="mb-2 block text-[13.5px] font-medium text-muted">Support email</span>
                      <input bind:value={settingsForm.supportEmail} type="email" placeholder="support@acme.com" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] lowercase text-fg placeholder:text-faint outline-none focus:border-violet" />
                    </label>
                  </div>
                </div>
                <div class="flex items-center gap-3">
                  <button type="submit" disabled={savingSettings} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{savingSettings ? 'Saving…' : 'Save changes'}</button>
                  <button type="button" onclick={seedSettings} class="rounded-lg px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Reset</button>
                </div>
              </div>

              <div class="lg:sticky lg:top-8">
                <div class="rounded-2xl border border-line bg-surface p-5">
                  <div class="mb-3 text-[13px] text-faint">Borrower link preview</div>
                  <div class="overflow-hidden rounded-xl border border-line bg-bg/40">
                    <div class="flex items-center gap-2.5 border-b border-line px-4 py-3 text-[14px] font-semibold text-fg">
                      <span class="grid h-6 w-6 place-items-center rounded-md text-[12px] font-bold" style="background:{settingsForm.brandColor};color:{settingsForm.brandTextColor}">{(settingsForm.brandName || settingsForm.orgName || 'K').trim()[0] || 'K'}</span>
                      {settingsForm.brandName || settingsForm.orgName || 'Your brand'}
                    </div>
                    <div class="px-4 py-5">
                      <div class="text-[15px] font-medium leading-snug text-fg">{settingsForm.brandName || settingsForm.orgName || 'A lender'} requested your income check</div>
                      <p class="mt-1.5 text-[13px] text-muted">Upload your statements — we only ever share your score.</p>
                      <div class="mt-4 inline-flex rounded-lg px-4 py-2 text-[14px] font-semibold" style="background:{settingsForm.brandColor};color:{settingsForm.brandTextColor}">Upload statements</div>
                    </div>
                  </div>
                  <p class="mt-3 text-[12px] leading-relaxed text-faint">The accent colour tints buttons, links and highlights; the text colour keeps them readable.</p>
                </div>
              </div>
            </form>

          {:else if settingsTab === 'products'}
            <form onsubmit={saveSettings} class="mt-6 space-y-5">
              <div class="rounded-2xl border border-line bg-surface p-6 lg:p-7">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-[15px] font-medium text-fg">Loan products</div>
                    <p class="mt-1 text-[13.5px] text-muted">Borrowers pick one on the link page; it caps the amount they can request.</p>
                  </div>
                  <button type="button" onclick={addProduct} class="shrink-0 rounded-lg border border-line px-3.5 py-2 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg">+ Add</button>
                </div>
                <div class="mt-4 space-y-3">
                  {#each settingsForm.loanProducts as p, i (i)}
                    <div class="flex items-end gap-3 rounded-xl border border-line bg-bg/30 p-3">
                      <label class="flex-1"><span class="mb-1.5 block text-[12.5px] text-faint">Max amount (₦)</span><input bind:value={p.maxAmount} type="number" min="0" placeholder="100000" class="w-full rounded-lg border border-line bg-bg/40 px-3.5 py-2.5 text-[15px] text-fg outline-none focus:border-violet" /></label>
                      <label class="w-24"><span class="mb-1.5 block text-[12.5px] text-faint">Interest (%)</span><input bind:value={p.interestRate} type="number" min="0" step="0.1" placeholder="5" class="w-full rounded-lg border border-line bg-bg/40 px-3.5 py-2.5 text-[15px] text-fg outline-none focus:border-violet" /></label>
                      <label class="w-28"><span class="mb-1.5 block text-[12.5px] text-faint">Tenor (days)</span><input bind:value={p.tenorDays} type="number" min="0" placeholder="30" class="w-full rounded-lg border border-line bg-bg/40 px-3.5 py-2.5 text-[15px] text-fg outline-none focus:border-violet" /></label>
                      <button type="button" onclick={() => removeProduct(i)} aria-label="Remove" class="mb-1 grid h-10 w-10 shrink-0 place-items-center rounded-lg text-faint transition-colors hover:text-red-400"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14"/></svg></button>
                    </div>
                  {:else}
                    <p class="rounded-xl border border-dashed border-line px-4 py-5 text-center text-[14px] text-faint">No loan products yet. Add one so borrowers see options and a max amount.</p>
                  {/each}
                </div>
              </div>
              <div class="flex items-center gap-3">
                <button type="submit" disabled={savingSettings} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{savingSettings ? 'Saving…' : 'Save changes'}</button>
                <button type="button" onclick={seedSettings} class="rounded-lg px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Reset</button>
              </div>
            </form>

          {:else if settingsTab === 'rules'}
            <form onsubmit={saveSettings} class="mt-6 space-y-5">
              <div class="rounded-2xl border border-line bg-surface p-6 lg:p-7">
                <div class="text-[15px] font-medium text-fg">Lending rules</div>
                <p class="mt-1 text-[13.5px] text-muted">Kova auto-declines any application scoring below this threshold. Leave blank to use the platform default of 40.</p>
                <label class="mt-4 block max-w-[240px]">
                  <span class="mb-2 block text-[13.5px] font-medium text-muted">Auto-decline below score</span>
                  <div class="flex items-center gap-3">
                    <input bind:value={settingsForm.minScore} type="number" min="0" max="100" placeholder="40" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[15px] text-fg placeholder:text-faint outline-none focus:border-violet" />
                    <span class="text-[13px] text-faint">/ 100</span>
                  </div>
                </label>
              </div>
              <div class="flex items-center gap-3">
                <button type="submit" disabled={savingSettings} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{savingSettings ? 'Saving…' : 'Save changes'}</button>
                <button type="button" onclick={seedSettings} class="rounded-lg px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Reset</button>
              </div>
            </form>
          {/if}
        {/if}
      </div>
    </main>
  </div>

  <!-- CREATE MODAL -->
  {#if modal}
    <div transition:fade={{ duration: 160 }} class="fixed inset-0 z-40 grid place-items-center bg-black/60 p-4 backdrop-blur-sm" onclick={() => (modal = null)} role="presentation">
      <div transition:scale={{ duration: 200, start: 0.96 }} class="w-full max-w-[460px] rounded-2xl border border-line bg-surface p-7 shadow-2xl" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        {#if modal === 'key'}
          <h2 class="text-[22px] font-medium tracking-tight text-fg">Create API key</h2>
          <p class="mt-1.5 text-[15px] text-muted">We will generate a publishable and a secret key for your workspace.</p>
          <form onsubmit={submitKey} class="mt-6 space-y-4">
            <label class="block">
              <span class="mb-2 block text-[14px] font-medium text-fg">Key name</span>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={formName} autofocus placeholder="e.g. Production server" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[16px] text-fg placeholder:text-faint outline-none focus:border-violet" />
            </label>
            {#if formErr}<p class="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-[14px] text-red-300">{formErr}</p>{/if}
            <div class="flex justify-end gap-3 pt-1">
              <button type="button" onclick={() => (modal = null)} class="rounded-lg px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Cancel</button>
              <button type="submit" disabled={busy} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{busy ? 'Creating…' : 'Create key'}</button>
            </div>
          </form>
        {:else if modal === 'link'}
          <h2 class="text-[22px] font-medium tracking-tight text-fg">Create shareable link</h2>
          <p class="mt-1.5 text-[15px] text-muted">The borrower picks a loan option, enters their details, and uploads statements; we score them and email the offer.</p>
          <form onsubmit={submitLink} class="mt-6 space-y-4">
            <label class="block">
              <span class="mb-2 block text-[14px] font-medium text-fg">Note</span>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={formNote} autofocus placeholder="e.g. John Doe · loan enquiry" class="w-full rounded-xl border border-line bg-bg/40 px-4 py-3 text-[16px] text-fg placeholder:text-faint outline-none focus:border-violet" />
            </label>
            {#if !(workspace?.loanProducts || []).length}
              <p class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-[13.5px] text-amber-300">Tip: add loan products in <button type="button" onclick={() => { modal = null; go('settings'); }} class="underline">Settings</button> so borrowers see options to choose from.</p>
            {/if}
            {#if formErr}<p class="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-[14px] text-red-300">{formErr}</p>{/if}
            <div class="flex justify-end gap-3 pt-1">
              <button type="button" onclick={() => (modal = null)} class="rounded-lg px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Cancel</button>
              <button type="submit" disabled={busy} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{busy ? 'Creating…' : 'Create link'}</button>
            </div>
          </form>
        {/if}
      </div>
    </div>
  {/if}

  <!-- CREATED (secret reveal / link share) -->
  {#if created}
    <div transition:fade={{ duration: 160 }} class="fixed inset-0 z-40 grid place-items-center bg-black/60 p-4 backdrop-blur-sm" onclick={() => (created = null)} role="presentation">
      <div transition:scale={{ duration: 200, start: 0.96 }} class="w-full max-w-[540px] rounded-2xl border border-line bg-surface p-7 shadow-2xl" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="flex items-center gap-3">
          <span class="grid h-10 w-10 place-items-center rounded-full bg-accent/15 text-accent">{@html icons.check}</span>
          <h2 class="text-[22px] font-medium tracking-tight text-fg">{created.type === 'key' ? 'API key created' : 'Link ready to share'}</h2>
        </div>

        {#if created.type === 'key'}
          <p class="mt-4 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-[14px] text-amber-300">Copy your secret key now. For your security it will not be shown in full again.</p>
          <div class="mt-5 space-y-3 font-mono text-[14px]">
            <div>
              <div class="mb-1.5 text-[13px] text-faint">Publishable key</div>
              <div class="flex items-center gap-2.5"><code class="flex-1 truncate rounded-lg bg-bg/60 px-3.5 py-2.5 text-muted">{created.data.publishable}</code><button onclick={() => copy(created.data.publishable, 'Publishable key copied')} class="text-faint transition-colors hover:text-fg">{@html icons.copy}</button></div>
            </div>
            <div>
              <div class="mb-1.5 text-[13px] text-faint">Secret key</div>
              <div class="flex items-center gap-2.5"><code class="flex-1 truncate rounded-lg bg-bg/60 px-3.5 py-2.5 text-muted">{created.data.secret}</code><button onclick={() => copy(created.data.secret, 'Secret key copied')} class="text-faint transition-colors hover:text-fg">{@html icons.copy}</button></div>
            </div>
          </div>
        {:else}
          <p class="mt-4 text-[15px] text-muted">Share the borrower link below. You can track the result under Links.</p>
          <div class="mt-5 space-y-3 font-mono text-[14px]">
            <div>
              <div class="mb-1.5 text-[13px] text-faint">Borrower link</div>
              <div class="flex items-center gap-2.5"><code class="flex-1 truncate rounded-lg bg-bg/60 px-3.5 py-2.5 text-muted">{created.data.borrowerUrl}</code><button onclick={() => copy(created.data.borrowerUrl, 'Link copied to clipboard')} class="text-faint transition-colors hover:text-fg">{@html icons.copy}</button></div>
            </div>
          </div>
        {/if}

        <div class="mt-6 flex justify-end gap-3">
          {#if created.type === 'link'}
            <a href={created.data.viewUrl} target="_blank" class="rounded-lg border border-line px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Open lender view</a>
          {/if}
          <button onclick={() => (created = null)} class="rounded-lg bg-fg px-5 py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90">Done</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- LINK DETAILS -->
  {#if detail}
    <div transition:fade={{ duration: 160 }} class="fixed inset-0 z-40 flex justify-end bg-black/60 backdrop-blur-sm" onclick={() => (detail = null)} role="presentation">
      <div transition:fly={{ x: 40, duration: 220 }} class="h-full w-full max-w-[720px] overflow-y-auto border-l border-line bg-surface p-7 shadow-2xl" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="flex items-start justify-between gap-4">
          <div>
            <div class="flex items-center gap-2.5">
              <span class="rounded-md px-2.5 py-1 text-[13px] capitalize {detail.link?.repaid ? 'bg-accent/15 text-accent' : detail.status === 'declined' || detail.status === 'rejected' ? 'bg-red-500/15 text-red-400' : 'bg-surface-2 text-muted'}">{detail.link?.repaid ? 'repaid' : detail.status}</span>
              <span class="text-[13px] text-faint">{fmtDate(detail.link.createdAt)}</span>
            </div>
            <h2 class="mt-2 text-[24px] font-medium tracking-tight text-fg">{detail.link.note || 'Untitled request'}</h2>
          </div>
          <button onclick={() => (detail = null)} aria-label="Close" class="shrink-0 text-faint transition-colors hover:text-fg">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <!-- share links -->
        <div class="mt-6 space-y-3">
          <div>
            <div class="mb-1.5 text-[13px] text-faint">Borrower link</div>
            <div class="flex items-center gap-2.5"><code class="flex-1 truncate rounded-lg bg-bg/60 px-3.5 py-2.5 font-mono text-[13.5px] text-muted">{detail.link.borrowerUrl}</code><button onclick={() => copy(detail.link.borrowerUrl, 'Link copied to clipboard')} class="text-faint transition-colors hover:text-fg">{@html icons.copy}</button></div>
          </div>
          <div>
            <div class="mb-1.5 text-[13px] text-faint">Lender view (score only)</div>
            <div class="flex items-center gap-2.5"><code class="flex-1 truncate rounded-lg bg-bg/60 px-3.5 py-2.5 font-mono text-[13.5px] text-muted">{detail.link.viewUrl}</code><a href={detail.link.viewUrl} target="_blank" class="text-faint transition-colors hover:text-fg" aria-label="Open">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><path d="M15 3h6v6M10 14 21 3"/></svg>
            </a></div>
          </div>
        </div>

        <div class="my-6 h-px bg-line"></div>

        <!-- borrower + loan decision -->
        {#if detail.link.borrowerName || detail.link.amountRequested || detail.link.decision}
          <div class="rounded-xl border border-line bg-bg/40 p-5">
            {#if detail.link.borrowerName}
              <div class="flex items-center justify-between text-[14px]">
                <span class="text-faint">Borrower</span>
                <span class="text-fg">{detail.link.borrowerName}{#if detail.link.borrowerEmail} · <span class="lowercase text-muted">{detail.link.borrowerEmail}</span>{/if}</span>
              </div>
            {/if}
            <div class="mt-2.5 flex items-center justify-between text-[14px]">
              <span class="text-faint">Requested</span>
              <span class="text-fg">{detail.link.amountRequested ? naira(detail.link.amountRequested) : '—'}</span>
            </div>
            {#if detail.link.maxAmount}
              <div class="mt-2.5 flex items-center justify-between text-[14px]"><span class="text-faint">Lender cap</span><span class="text-fg">{naira(detail.link.maxAmount)}</span></div>
            {/if}
            {#if detail.link.interestRate || detail.link.tenorDays}
              <div class="mt-2.5 flex items-center justify-between text-[14px]"><span class="text-faint">Terms</span><span class="text-fg">{detail.link.interestRate ? detail.link.interestRate + '%' : ''}{detail.link.interestRate && detail.link.tenorDays ? ' · ' : ''}{detail.link.tenorDays ? detail.link.tenorDays + ' days' : ''}</span></div>
            {/if}
            {#if detail.link.decision}
              <div class="mt-3 flex items-center justify-between border-t border-line/60 pt-3">
                <span class="text-[14px] text-faint">Decision</span>
                <span class="text-[15px] font-semibold {decisionColor(detail.link.decision)}">{decisionLabel[detail.link.decision] || detail.link.decision}{#if detail.link.decision !== 'declined'} · {naira(detail.link.offerAmount)}{/if}</span>
              </div>
            {/if}

            {#if detail.link.disbursed}
              <div class="mt-4 rounded-xl border border-line bg-bg/40 p-5">
                <ol class="relative ml-1 space-y-5 border-l border-line pl-6">
                  <li class="relative">
                    <span class="absolute -left-[31px] top-0 grid h-5 w-5 place-items-center rounded-full bg-accent text-bg">{@html icons.check}</span>
                    <div class="text-[14px] font-medium text-fg">Loan disbursed</div>
                    <div class="mt-0.5 text-[13px] text-muted">{naira(detail.link.offerAmount)} to {detail.link.accountName || detail.link.borrowerName || 'borrower'}</div>
                    {#if detail.link.disbursedAt}<div class="mt-0.5 text-[12.5px] text-faint">{fmtDate(detail.link.disbursedAt)}</div>{/if}
                  </li>
                  <li class="relative">
                    <span class="absolute -left-[31px] top-0 grid h-5 w-5 place-items-center rounded-full border-2 {detail.link.repaid ? 'border-accent bg-accent/20' : 'border-violet bg-violet/20'}"></span>
                    <div class="text-[14px] font-medium text-fg">Repayment {detail.link.repaid ? 'was due' : 'due'}</div>
                    <div class="mt-0.5 text-[13px] text-muted">{naira(detail.link.repaymentTotal || detail.link.offerAmount)}</div>
                    {#if detail.link.repaymentDueAt}<div class="mt-0.5 text-[12.5px] text-faint">{fmtDate(detail.link.repaymentDueAt)}</div>{/if}
                  </li>
                  <li class="relative">
                    {#if detail.link.repaid}
                      <span class="absolute -left-[31px] top-0 grid h-5 w-5 place-items-center rounded-full bg-accent text-bg">{@html icons.check}</span>
                      <div class="text-[14px] font-medium text-accent">Repaid in full</div>
                      {#if detail.link.repaidAt}<div class="mt-0.5 text-[12.5px] text-faint">{fmtDate(detail.link.repaidAt)}</div>{/if}
                    {:else}
                      <span class="absolute -left-[31px] top-0 grid h-5 w-5 place-items-center rounded-full border-2 border-dashed border-faint bg-surface"></span>
                      <div class="text-[14px] font-medium text-fg">Awaiting repayment</div>
                      <div class="mt-3 flex flex-wrap gap-2.5">
                        <button onclick={() => requestRepayment(detail.link)} disabled={requestingRepay} class="rounded-lg bg-fg px-4 py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{requestingRepay ? 'Sending…' : 'Send repayment link'}</button>
                        <button onclick={() => verifyRepayment(detail.link)} disabled={verifyingRepay} class="rounded-lg border border-line px-4 py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-60">{verifyingRepay ? 'Checking…' : 'Check payment'}</button>
                        <button onclick={() => markRepaid(detail.link)} disabled={repaying} class="rounded-lg border border-line px-4 py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-60">{repaying ? 'Saving…' : 'Mark repaid'}</button>
                      </div>
                      <p class="mt-2 text-[12.5px] text-faint">Emails the borrower a secure Monnify link to pay (also auto-sent on the due date). "Check payment" confirms with Monnify once they've paid.</p>
                    {/if}
                  </li>
                </ol>
              </div>
            {:else if detail.status === 'rejected' || detail.link.status === 'rejected'}
              <div class="mt-4 flex items-center gap-2 rounded-lg bg-red-500/10 px-3.5 py-2.5 text-[14px] text-red-400"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg> Rejected by lender</div>
            {:else if detail.link.decision === 'declined' || detail.status === 'declined'}
              <div class="mt-4 flex items-center gap-2 rounded-lg bg-red-500/10 px-3.5 py-2.5 text-[14px] text-red-400"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg> Auto-declined — score below the lending threshold</div>
            {:else if detail.link.accepted}
              <div class="mt-4">
                <div class="mb-2.5 text-[13.5px] text-muted">Borrower accepted{#if detail.link.accountName} · verified as <span class="text-fg">{detail.link.accountName}</span>{/if}</div>
                {#if otpRequired}
                  <div class="rounded-lg border border-line bg-surface/60 p-4">
                    <div class="text-[14px] text-fg">Enter the OTP</div>
                    <div class="mt-1 text-[13px] text-muted">Monnify emailed a one-time code to your registered email to authorize this payout of <b>{naira(detail.link.offerAmount)}</b>.</div>
                    <input inputmode="numeric" autocomplete="one-time-code" bind:value={otpCode} placeholder="123456" class="mt-3 w-full rounded-lg border border-line bg-bg px-3.5 py-2.5 text-[15px] tracking-widest text-fg outline-none focus:border-faint" />
                    <div class="mt-3 flex gap-2.5">
                      <button onclick={() => authorizeDisburse(detail.link)} disabled={authorizing} class="flex-1 rounded-lg bg-fg py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{authorizing ? 'Authorizing…' : 'Authorize payout'}</button>
                      <button onclick={() => resendOtp(detail.link)} disabled={authorizing} class="rounded-lg border border-line px-4 py-2.5 text-[14px] text-muted transition-colors hover:text-fg">Resend code</button>
                    </div>
                  </div>
                {:else if confirmDisburse}
                  <div class="rounded-lg border border-line bg-surface/60 p-4">
                    <div class="text-[14px] text-fg">Disburse <b>{naira(detail.link.offerAmount)}</b> to {detail.link.accountName || detail.link.borrowerName || 'the borrower'}?</div>
                    <div class="mt-3 flex gap-2.5">
                      <button onclick={() => doDisburse(detail.link)} disabled={disbursing} class="flex-1 rounded-lg bg-fg py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{disbursing ? 'Disbursing…' : 'Confirm disbursement'}</button>
                      <button onclick={() => (confirmDisburse = false)} disabled={disbursing} class="rounded-lg border border-line px-4 py-2.5 text-[14px] text-muted transition-colors hover:text-fg">Cancel</button>
                    </div>
                  </div>
                {:else if confirmReject}
                  <div class="rounded-lg border border-red-500/30 bg-red-500/5 p-4">
                    <div class="text-[14px] text-fg">Reject this application? The borrower will be notified with your reason and can no longer proceed.</div>
                    <textarea bind:value={rejectReason} rows="2" placeholder="Reason (shared with the borrower) — e.g. income too irregular for this amount" class="mt-3 w-full resize-none rounded-lg border border-line bg-bg px-3.5 py-2.5 text-[14px] text-fg placeholder:text-faint outline-none focus:border-faint"></textarea>
                    <div class="mt-3 flex gap-2.5">
                      <button onclick={() => doReject(detail.link)} class="flex-1 rounded-lg bg-red-500/90 py-2.5 text-[14px] font-semibold text-white transition-opacity hover:opacity-90">Reject &amp; notify borrower</button>
                      <button onclick={() => (confirmReject = false)} class="rounded-lg border border-line px-4 py-2.5 text-[14px] text-muted transition-colors hover:text-fg">Cancel</button>
                    </div>
                  </div>
                {:else}
                  <button onclick={() => (confirmDisburse = true)} class="w-full rounded-lg bg-fg py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90">Disburse {naira(detail.link.offerAmount)}</button>
                  <button onclick={() => (confirmReject = true)} class="mt-2 w-full text-center text-[13.5px] text-muted transition-colors hover:text-red-400">Reject application</button>
                {/if}
              </div>
            {:else if detail.link.decision === 'approved' || detail.link.decision === 'counter'}
              <div class="mt-4 rounded-lg border border-line bg-surface/60 px-3.5 py-2.5 text-[13.5px] text-muted">Waiting for the borrower to apply and confirm their payout account.</div>
              {#if confirmReject}
                <div class="mt-3 rounded-lg border border-red-500/30 bg-red-500/5 p-4">
                  <div class="text-[14px] text-fg">Reject this application? The borrower will be notified with your reason and can no longer proceed.</div>
                  <textarea bind:value={rejectReason} rows="2" placeholder="Reason (shared with the borrower) — e.g. income too irregular for this amount" class="mt-3 w-full resize-none rounded-lg border border-line bg-bg px-3.5 py-2.5 text-[14px] text-fg placeholder:text-faint outline-none focus:border-faint"></textarea>
                  <div class="mt-3 flex gap-2.5">
                    <button onclick={() => doReject(detail.link)} class="flex-1 rounded-lg bg-red-500/90 py-2.5 text-[14px] font-semibold text-white transition-opacity hover:opacity-90">Reject &amp; notify borrower</button>
                    <button onclick={() => (confirmReject = false)} class="rounded-lg border border-line px-4 py-2.5 text-[14px] text-muted transition-colors hover:text-fg">Cancel</button>
                  </div>
                </div>
              {:else}
                <button onclick={() => resendOffer(detail.link)} disabled={resendingOffer} class="mt-3 w-full rounded-lg border border-line py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-60">{resendingOffer ? 'Sending…' : 'Resend offer email'}</button>
                <button onclick={() => (confirmReject = true)} class="mt-2 w-full text-center text-[13.5px] text-muted transition-colors hover:text-red-400">Reject application</button>
              {/if}
            {/if}
          </div>
        {/if}

        {#if detail.loading}
          <div class="flex items-center gap-3 py-8 text-[15px] text-muted"><span class="h-4 w-4 animate-spin rounded-full border-2 border-line border-t-violet"></span> Loading result…</div>
        {:else if detail.report}
          {@const s = detail.report.score}
          {@const f = s.features || {}}
          {@const files = detail.report.files || []}
          {@const comps = s.components || {}}
          <h3 class="mb-4 text-[16px] font-medium text-fg">Statement analysis</h3>
          <div class="flex items-center gap-6">
            <div class="relative h-[128px] w-[128px] shrink-0">
              <svg viewBox="0 0 100 100" class="h-full w-full -rotate-90">
                <circle cx="50" cy="50" r="42" fill="none" stroke="var(--color-line)" stroke-width="7" />
                <circle cx="50" cy="50" r="42" fill="none" stroke={s.band === 'A' ? 'var(--color-accent)' : 'var(--color-violet)'} stroke-width="7" stroke-linecap="round" stroke-dasharray={2 * Math.PI * 42} stroke-dashoffset={2 * Math.PI * 42 * (1 - (s.score || 0) / 100)} style="transition:stroke-dashoffset 1s ease" />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center">
                <span class="text-[34px] font-light leading-none text-fg">{s.score}</span>
                <span class="mt-1 text-[12px] text-faint">band {s.band}</span>
              </div>
            </div>
            <div class="grid flex-1 grid-cols-2 gap-x-6 gap-y-3">
              <div><div class="text-[13px] text-faint">Confidence</div><div class="text-[17px] font-medium text-fg">{Math.round((s.confidence || 0) * 100)}%</div></div>
              <div><div class="text-[13px] text-faint">Recommended limit</div><div class="text-[17px] font-medium text-fg">{nairaN(s.limitRecommendation)}</div></div>
              <div><div class="text-[13px] text-faint">Avg monthly inflow</div><div class="text-[17px] font-medium text-fg">{nairaN(f.avgMonthlyInflow)}</div></div>
              <div><div class="text-[13px] text-faint">Total inflow</div><div class="text-[17px] font-medium text-fg">{nairaN(f.totalInflow)}</div></div>
              <div><div class="text-[13px] text-faint">Months covered</div><div class="text-[17px] font-medium text-fg">{f.monthsCovered != null ? Math.round(f.monthsCovered) : '—'}</div></div>
              <div><div class="text-[13px] text-faint">Transactions</div><div class="text-[17px] font-medium text-fg">{f.transactionCount ?? files.reduce((a, x) => a + (x.transactions || 0), 0)}</div></div>
            </div>
          </div>

          <!-- statements (individual) -->
          <h3 class="mt-7 text-[15px] font-medium text-fg">Statements analysed</h3>
          <div class="mt-3 overflow-hidden rounded-xl border border-line">
            <table class="w-full text-left text-[14px]">
              <thead class="border-b border-line bg-surface/60 font-mono text-[11px] uppercase tracking-wider text-faint"><tr><th class="px-4 py-2.5 font-normal">Bank</th><th class="px-4 py-2.5 font-normal text-right">Transactions</th></tr></thead>
              <tbody>
                {#each files as file (file.filename)}
                  <tr class="border-b border-line/60 last:border-0">
                    <td class="px-4 py-2.5 font-medium text-fg">{file.bank || '—'}{#if !file.parsed}<span class="ml-2 rounded bg-red-500/15 px-1.5 py-0.5 text-[11px] text-red-400">unreadable</span>{/if}</td>
                    <td class="px-4 py-2.5 text-right text-muted">{file.transactions || 0}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>

          <!-- accounts + banks -->
          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <div class="rounded-xl border border-line bg-bg/40 p-4"><div class="text-[13px] text-faint">Account holder{(detail.report.accounts || []).length > 1 ? 's' : ''}</div><div class="mt-1 text-[15px] text-fg">{(detail.report.accounts || []).join(', ') || '—'}</div></div>
            <div class="rounded-xl border border-line bg-bg/40 p-4"><div class="text-[13px] text-faint">Banks</div><div class="mt-1 text-[15px] text-fg">{(detail.report.banks || []).join(', ') || '—'}</div></div>
          </div>

          <!-- component (individual) scores -->
          {#if Object.keys(comps).length}
            <h3 class="mt-7 text-[15px] font-medium text-fg">Score breakdown</h3>
            <div class="mt-3 space-y-2.5">
              {#each Object.entries(comps) as [name, val] (name)}
                <div class="flex items-center gap-3">
                  <span class="w-24 shrink-0 text-[13.5px] capitalize text-muted">{name}</span>
                  <div class="h-2 flex-1 overflow-hidden rounded-full bg-surface-2"><div class="h-full rounded-full bg-violet" style="width:{Math.max(0, Math.min(100, val))}%"></div></div>
                  <span class="w-9 shrink-0 text-right text-[13.5px] tabular-nums text-fg">{Math.round(val)}</span>
                </div>
              {/each}
            </div>
          {/if}

          {#if s.reasons?.length}
            <h3 class="mt-7 text-[15px] font-medium text-fg">Why this score</h3>
            <ul class="mt-3 space-y-0">
              {#each s.reasons.slice(0, 6) as reason (reason.detail)}
                <li class="flex items-start gap-3 border-t border-line/60 py-3 text-[14px] text-muted">
                  <span class="mt-1 h-3.5 w-1.5 shrink-0 rounded-full" style="background:{reason.impact === 'positive' ? 'var(--color-accent)' : '#ef4444'}"></span>
                  {reason.detail}
                </li>
              {/each}
            </ul>
          {/if}
        {:else}
          <div class="rounded-xl border border-line bg-bg/40 p-6 text-center">
            <div class="mx-auto mb-3 h-9 w-9 animate-spin rounded-full border-2 border-line border-t-violet"></div>
            <div class="text-[15px] text-fg">Waiting for the borrower to upload</div>
            <p class="mt-1.5 text-[14px] text-muted">Share the borrower link above. The score appears here automatically once they submit their statements.</p>
            <button onclick={() => copy(detail.link.borrowerUrl, 'Link copied to clipboard')} class="mt-4 rounded-lg border border-line px-4 py-2 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg">Copy borrower link</button>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Repayment-only side pane -->
  {#if repayDetail}
    {@const rst = repaymentStatus(repayDetail)}
    <div transition:fade={{ duration: 160 }} class="fixed inset-0 z-40 flex justify-end bg-black/60 backdrop-blur-sm" onclick={() => (repayDetail = null)} role="presentation">
      <div transition:fly={{ x: 40, duration: 220 }} class="h-full w-full max-w-[440px] overflow-y-auto border-l border-line bg-surface p-7 shadow-2xl" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="flex items-start justify-between gap-4">
          <div>
            <span class="text-[13px] font-medium capitalize {repayColor(rst)}">{rst}</span>
            <h2 class="mt-1.5 text-[24px] font-medium tracking-tight text-fg">{repayDetail.borrowerName || repayDetail.note || 'Repayment'}</h2>
            {#if repayDetail.borrowerEmail}<p class="mt-0.5 text-[13.5px] lowercase text-muted">{repayDetail.borrowerEmail}</p>{/if}
          </div>
          <button onclick={() => (repayDetail = null)} aria-label="Close" class="shrink-0 text-faint transition-colors hover:text-fg">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="mt-6 rounded-xl border border-line bg-bg/40 p-4">
          <div class="flex items-center justify-between py-1 text-[14px]"><span class="text-faint">Principal</span><span class="text-fg">{naira(repayDetail.offerAmount)}</span></div>
          <div class="flex items-center justify-between py-1 text-[14px]"><span class="text-faint">Total due</span><span class="font-medium text-fg">{naira(repayDetail.repaymentTotal || repayDetail.offerAmount)}</span></div>
          <div class="flex items-center justify-between py-1 text-[14px]"><span class="text-faint">Due date</span><span class="text-fg">{repayDetail.repaymentDueAt ? fmtDate(repayDetail.repaymentDueAt) : '—'}</span></div>
          <div class="flex items-center justify-between py-1 text-[14px]"><span class="text-faint">Status</span><span class="font-medium capitalize {repayColor(rst)}">{rst}</span></div>
        </div>

        {#if repayDetail.repaid}
          <div class="mt-5 flex items-center gap-2 rounded-lg bg-accent/10 px-3.5 py-3 text-[14px] text-accent"><span>{@html icons.check}</span> This loan has been repaid.</div>
        {:else}
          <div class="mt-5 space-y-2.5">
            <button onclick={() => requestRepayment(repayDetail)} disabled={requestingRepay} class="w-full rounded-lg bg-fg py-2.5 text-[14px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">{requestingRepay ? 'Sending…' : 'Send repayment link'}</button>
            <button onclick={() => verifyRepayment(repayDetail)} disabled={verifyingRepay} class="w-full rounded-lg border border-line py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-60">{verifyingRepay ? 'Checking Monnify…' : 'Check payment status'}</button>
            <button onclick={() => markRepaid(repayDetail)} disabled={repaying} class="w-full rounded-lg border border-line py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg disabled:opacity-60">{repaying ? 'Saving…' : 'Mark as repaid'}</button>
          </div>
          {#if repayStatusMsg}<p class="mt-3 text-[13px] text-faint">{repayStatusMsg}</p>{/if}
          <p class="mt-3 text-[12.5px] leading-relaxed text-faint">"Send repayment link" emails the borrower a secure Monnify link (also auto-sent on the due date). "Check payment status" confirms with Monnify and marks repaid once they've paid.</p>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Activity detail side pane -->
  {#if activityDetail}
    <div transition:fade={{ duration: 160 }} class="fixed inset-0 z-40 flex justify-end bg-black/60 backdrop-blur-sm" onclick={() => (activityDetail = null)} role="presentation">
      <div transition:fly={{ x: 40, duration: 220 }} class="h-full w-full max-w-[440px] overflow-y-auto border-l border-line bg-surface p-7 shadow-2xl" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="flex items-start justify-between gap-4">
          <div>
            <span class="rounded-md bg-surface-2 px-2.5 py-1 text-[13px] capitalize text-muted">{activityDetail.actor}</span>
            <h2 class="mt-2 text-[24px] font-medium tracking-tight text-fg">{auditLabel[activityDetail.action] || activityDetail.action}</h2>
          </div>
          <button onclick={() => (activityDetail = null)} aria-label="Close" class="shrink-0 text-faint transition-colors hover:text-fg">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="mt-6 rounded-xl border border-line bg-bg/40 p-4">
          <div class="flex items-center justify-between py-1.5 text-[14px]"><span class="text-faint">Event</span><span class="font-mono text-[13px] text-fg">{activityDetail.action}</span></div>
          <div class="flex items-center justify-between py-1.5 text-[14px]"><span class="text-faint">Actor</span><span class="capitalize text-fg">{activityDetail.actor}</span></div>
          <div class="flex items-center justify-between py-1.5 text-[14px]"><span class="text-faint">When</span><span class="text-fg">{fmtDate(activityDetail.createdAt)}</span></div>
          {#if activityDetail.target}<div class="flex items-center justify-between gap-3 py-1.5 text-[14px]"><span class="shrink-0 text-faint">Request</span><span class="truncate font-mono text-[12.5px] text-muted">{activityDetail.target}</span></div>{/if}
        </div>

        {#if activityDetail.detail}
          <div class="mt-4">
            <div class="mb-1.5 text-[13px] text-faint">Detail</div>
            <div class="rounded-lg bg-bg/60 px-3.5 py-3 text-[14px] text-muted">{activityDetail.detail}</div>
          </div>
        {/if}

        {#if activityDetail.target}
          {@const rel = links.find((l) => l.id === activityDetail.target)}
          {#if rel}
            <button onclick={() => { activityDetail = null; openDetail(rel); }} class="mt-5 w-full rounded-lg border border-line py-2.5 text-[14px] font-medium text-muted transition-colors hover:border-faint hover:text-fg">Open this request</button>
          {/if}
        {/if}
      </div>
    </div>
  {/if}
{/if}
