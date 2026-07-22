<script>
  import { onMount } from 'svelte';

  let orgName = $state('');
  let useCase = $state('fintech');
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

  async function submit(e) {
    e.preventDefault();
    error = '';
    loading = true;
    try {
      const r = await fetch('/api/workspace', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ orgName, useCase }),
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
</script>

{#if ready}
  <form onsubmit={submit} class="space-y-6">
    <div>
      <label class="mb-2 block text-[13px] font-medium text-fg">Organisation name</label>
      <input
        bind:value={orgName}
        placeholder="Acme Lending"
        required
        class="w-full rounded-lg border border-line bg-surface px-3.5 py-2.5 text-[15px] text-fg placeholder:text-faint outline-none transition-colors focus:border-violet"
      />
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

    <button
      type="submit"
      disabled={loading}
      class="w-full rounded-lg bg-fg py-2.5 text-[15px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60"
    >
      {loading ? 'Creating…' : 'Create workspace'}
    </button>
  </form>
{:else}
  <div class="py-10 text-center text-[14px] text-muted">Loading…</div>
{/if}
