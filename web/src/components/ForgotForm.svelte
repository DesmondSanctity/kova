<script>
  let step = $state('email'); // 'email' | 'code'
  let email = $state('');
  let code = $state('');
  let password = $state('');
  let confirm = $state('');
  let loading = $state(false);
  let error = $state('');
  let devCode = $state('');
  let done = $state(false);

  async function requestCode(e) {
    e.preventDefault();
    error = '';
    loading = true;
    try {
      const r = await fetch('/auth/forgot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });
      const d = await r.json().catch(() => ({}));
      devCode = d.devCode || '';
      step = 'code';
    } catch {
      error = 'Network error. Please try again.';
    } finally {
      loading = false;
    }
  }

  async function submitReset(e) {
    e.preventDefault();
    error = '';
    if (password !== confirm) { error = 'Passwords do not match.'; return; }
    loading = true;
    try {
      const r = await fetch('/auth/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code, password }),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) { error = d.error || 'That code is invalid or expired.'; loading = false; return; }
      done = true;
      setTimeout(() => (location.href = '/login'), 1200);
    } catch {
      error = 'Network error.';
      loading = false;
    }
  }

  const field =
    'w-full rounded-xl border border-line bg-surface px-5 py-4 text-[19px] text-fg placeholder:text-faint outline-none transition-colors focus:border-violet';
  const button =
    'w-full rounded-xl bg-fg py-4 text-[19px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60';
</script>

{#if done}
  <div class="rounded-xl border border-accent/30 bg-accent/10 p-6 text-[18px] text-accent">
    Password updated. Redirecting you to sign in…
  </div>
{:else if step === 'email'}
  <form onsubmit={requestCode} class="space-y-6">
    <div>
      <label class="mb-2.5 block text-[17px] font-medium text-fg">Email</label>
      <input class={field} bind:value={email} type="email" placeholder="you@company.com" autocomplete="email" required />
    </div>
    {#if error}
      <p class="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-[15px] text-red-300">{error}</p>
    {/if}
    <button type="submit" disabled={loading} class={button}>
      {loading ? 'Sending…' : 'Send reset code'}
    </button>
  </form>
{:else}
  <form onsubmit={submitReset} class="space-y-6">
    <p class="text-[17px] leading-relaxed text-muted">
      Enter the 6-digit code we sent to <span class="text-fg">{email}</span> and choose a new password.
    </p>
    {#if devCode}
      <div class="rounded-xl border border-line bg-bg/50 p-4 text-[15px]">
        <div class="mb-1 font-mono text-[13px] uppercase tracking-wider text-faint">Dev code</div>
        <span class="font-mono text-[22px] tracking-[0.3em] text-violet">{devCode}</span>
      </div>
    {/if}
    <div>
      <label class="mb-2.5 block text-[17px] font-medium text-fg">Reset code</label>
      <input
        class="{field} text-center font-mono tracking-[0.5em]"
        bind:value={code}
        inputmode="numeric"
        maxlength="6"
        placeholder="000000"
        autocomplete="one-time-code"
        required
      />
    </div>
    <div>
      <label class="mb-2.5 block text-[17px] font-medium text-fg">New password</label>
      <input class={field} bind:value={password} type="password" placeholder="At least 8 characters" autocomplete="new-password" minlength="8" required />
    </div>
    <div>
      <label class="mb-2.5 block text-[17px] font-medium text-fg">Confirm password</label>
      <input class={field} bind:value={confirm} type="password" placeholder="Re-enter password" autocomplete="new-password" minlength="8" required />
    </div>
    {#if error}
      <p class="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-[15px] text-red-300">{error}</p>
    {/if}
    <button type="submit" disabled={loading} class={button}>
      {loading ? 'Updating…' : 'Update password'}
    </button>
    <button
      type="button"
      onclick={() => { step = 'email'; error = ''; }}
      class="w-full text-center text-[16px] text-muted transition-colors hover:text-fg"
    >
      Use a different email
    </button>
  </form>
{/if}
