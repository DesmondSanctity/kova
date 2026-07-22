<script>
  import { onMount } from 'svelte';

  let step = $state(1);
  let orgName = $state('');
  let useCase = $state('fintech');
  // Monnify connection (required — each lender disburses on their own account).
  let monBaseUrl = $state('https://sandbox.monnify.com');
  let monApiKey = $state('');
  let monSecret = $state('');
  let monContract = $state('');
  let monWallet = $state('');
  let error = $state('');
  let loading = $state(false);
  let ready = $state(false);

  onMount(async () => {
    try {
      const r = await fetch('/api/me');
      if (r.status === 401) { location.href = '/login'; return; }
      const me = await r.json();
      if (!me.needsOnboarding) { location.href = '/dashboard'; return; }
      ready = true;
    } catch {
      ready = true;
    }
  });

  function next(e) {
    e.preventDefault();
    error = '';
    if (!orgName.trim()) { error = 'Enter your organisation name.'; return; }
    step = 2;
  }

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (!monApiKey.trim() || !monSecret.trim() || !monContract.trim()) { error = 'Enter your Monnify API key, secret and contract code.'; return; }
    if (monWallet.trim().length !== 10) { error = 'Enter your 10-digit Monnify wallet account number.'; return; }
    loading = true;
    try {
      const r = await fetch('/api/workspace', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          orgName,
          useCase,
          monnify: {
            baseUrl: monBaseUrl.trim(),
            apiKey: monApiKey.trim(),
            secretKey: monSecret.trim(),
            contractCode: monContract.trim(),
            walletAccount: monWallet.trim(),
          },
        }),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) { error = d.error || 'Could not create workspace.'; loading = false; return; }
      location.href = '/dashboard';
    } catch {
      error = 'Network error.';
      loading = false;
    }
  }

  const cases = [
    {
      id: 'fintech',
      title: 'Fintech or lender',
      desc: 'A team building lending or credit into a product.',
      icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M3 21h18M5 21V7l7-4 7 4v14M9.5 10h.01M14.5 10h.01M9.5 14h.01M14.5 14h.01"/></svg>',
    },
    {
      id: 'individual',
      title: 'Individual lender',
      desc: 'You lend to people directly and want quick checks.',
      icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><circle cx="12" cy="8" r="4"/><path d="M4.5 21a7.5 7.5 0 0 1 15 0"/></svg>',
    },
  ];

  const inputCls = 'w-full rounded-lg border border-line bg-surface px-3.5 py-2.5 text-[15px] text-fg placeholder:text-faint outline-none transition-colors focus:border-violet';
</script>

{#if ready}
  <!-- step indicator -->
  <div class="mb-6 flex items-center gap-2 text-[12.5px] text-faint">
    <span class="{step === 1 ? 'text-violet' : 'text-muted'}">1. Workspace</span>
    <span class="h-px w-6 bg-line"></span>
    <span class="{step === 2 ? 'text-violet' : 'text-muted'}">2. Connect Monnify</span>
  </div>

  {#if step === 1}
    <form onsubmit={next} class="space-y-6">
      <div>
        <label class="mb-2 block text-[13px] font-medium text-fg">Organisation name</label>
        <input bind:value={orgName} placeholder="Acme Lending" required class={inputCls} />
      </div>

      <div>
        <span class="mb-2 block text-[13px] font-medium text-fg">How will you use Kova?</span>
        <div class="grid gap-3 sm:grid-cols-2">
          {#each cases as c (c.id)}
            <button
              type="button"
              onclick={() => (useCase = c.id)}
              class="relative rounded-xl border p-4 text-left transition-colors {useCase === c.id ? 'border-violet bg-violet/10' : 'border-line bg-surface hover:border-faint'}"
            >
              <span class="inline-grid h-8 w-8 place-items-center rounded-lg bg-surface-2 {useCase === c.id ? 'text-violet' : 'text-muted'}">{@html c.icon}</span>
              <div class="mt-3 text-[15px] font-medium text-fg">{c.title}</div>
              <div class="mt-1 text-[13px] leading-relaxed text-muted">{c.desc}</div>
              {#if useCase === c.id}
                <span class="absolute right-3 top-3 grid h-5 w-5 place-items-center rounded-full bg-violet text-[11px] font-semibold text-bg">✓</span>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      {#if error}
        <p class="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-300">{error}</p>
      {/if}

      <button type="submit" class="w-full rounded-lg bg-fg py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90">Continue</button>
    </form>
  {:else}
    <form onsubmit={submit} class="space-y-5">
      <div class="rounded-lg border border-line bg-surface-2/40 px-3.5 py-3 text-[13px] leading-relaxed text-muted">
        Kova disburses and collects on <b class="text-fg">your own Monnify account</b>, so funds and reconciliation stay with you. Find these in your Monnify dashboard under <span class="text-fg">Settings → API Keys &amp; Webhooks</span>. Your secret is encrypted at rest.
      </div>

      <div>
        <label class="mb-2 block text-[13px] font-medium text-fg">Environment</label>
        <select bind:value={monBaseUrl} class={inputCls}>
          <option value="https://sandbox.monnify.com">Sandbox</option>
          <option value="https://api.monnify.com">Live</option>
        </select>
      </div>
      <div>
        <label class="mb-2 block text-[13px] font-medium text-fg">API key</label>
        <input bind:value={monApiKey} placeholder="MK_PROD_XXXXXXXX" required class={inputCls} />
      </div>
      <div>
        <label class="mb-2 block text-[13px] font-medium text-fg">Secret key</label>
        <input bind:value={monSecret} type="password" placeholder="••••••••••••••••" required class={inputCls} />
      </div>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="mb-2 block text-[13px] font-medium text-fg">Contract code</label>
          <input bind:value={monContract} inputmode="numeric" placeholder="1234567890" required class={inputCls} />
        </div>
        <div>
          <label class="mb-2 block text-[13px] font-medium text-fg">Wallet account no.</label>
          <input bind:value={monWallet} inputmode="numeric" maxlength="10" placeholder="0123456789" required class={inputCls} />
        </div>
      </div>

      {#if error}
        <p class="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-300">{error}</p>
      {/if}

      <div class="flex gap-3">
        <button type="button" onclick={() => { error = ''; step = 1; }} class="rounded-lg border border-line px-4 py-2.5 text-[15px] text-muted transition-colors hover:text-fg">Back</button>
        <button type="submit" disabled={loading} class="flex-1 rounded-lg bg-fg py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60">
          {loading ? 'Verifying with Monnify…' : 'Connect & create workspace'}
        </button>
      </div>
    </form>
  {/if}
{:else}
  <div class="py-10 text-center text-[14px] text-muted">Loading…</div>
{/if}

