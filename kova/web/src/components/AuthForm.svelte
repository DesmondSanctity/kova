<script>
  let { mode = 'login' } = $props();

  let name = $state('');
  let email = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);

  const isSignup = $derived(mode === 'signup');

  async function submit(e) {
    e.preventDefault();
    error = '';
    loading = true;
    const path = isSignup ? '/auth/signup' : '/auth/login';
    const body = isSignup ? { name, email, password } : { email, password };
    try {
      const r = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const d = await r.json().catch(() => ({}));
      if (!r.ok) {
        error = d.error || 'Something went wrong. Please try again.';
        loading = false;
        return;
      }
      const me = await (await fetch('/api/me')).json();
      location.href = me.needsOnboarding ? '/onboarding' : '/dashboard';
    } catch {
      error = 'Network error. Is the server running?';
      loading = false;
    }
  }

  const field =
    'w-full rounded-xl border border-line bg-surface px-5 py-4 text-[19px] text-fg placeholder:text-faint outline-none transition-colors focus:border-violet';
</script>

<form onsubmit={submit} class="space-y-5">
  {#if isSignup}
    <div>
      <label class="mb-2.5 block text-[17px] font-medium text-fg">Full name</label>
      <input class={field} bind:value={name} placeholder="Ada Lovelace" autocomplete="name" required />
    </div>
  {/if}
  <div>
    <label class="mb-2.5 block text-[17px] font-medium text-fg">Email</label>
    <input class={field} bind:value={email} type="email" placeholder="you@company.com" autocomplete="email" required />
  </div>
  <div>
    <div class="mb-2.5 flex items-center justify-between">
      <label class="text-[17px] font-medium text-fg">Password</label>
      {#if !isSignup}
        <a href="/forgot" class="text-[16px] text-muted transition-colors hover:text-fg">Forgot password?</a>
      {/if}
    </div>
    <input
      class={field}
      bind:value={password}
      type="password"
      placeholder={isSignup ? 'At least 8 characters' : 'Your password'}
      autocomplete={isSignup ? 'new-password' : 'current-password'}
      minlength="8"
      required
    />
  </div>

  {#if error}
    <p class="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-300">{error}</p>
  {/if}

  <button
    type="submit"
    disabled={loading}
    class="w-full rounded-xl bg-fg py-4 text-[19px] font-semibold text-bg transition-opacity hover:opacity-90 disabled:opacity-60"
  >
    {loading ? 'Please wait…' : isSignup ? 'Create account' : 'Sign in'}
  </button>
</form>

<div class="my-6 flex items-center gap-3 text-[15px] text-faint">
  <span class="h-px flex-1 bg-line"></span> or <span class="h-px flex-1 bg-line"></span>
</div>

<a
  href="/auth/github"
  class="flex w-full items-center justify-center gap-3 rounded-xl border border-line bg-surface py-4 text-[19px] font-medium text-fg transition-colors hover:border-faint"
>
  <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a10 10 0 0 0-3.2 19.5c.5.1.7-.2.7-.5v-1.7c-2.8.6-3.4-1.3-3.4-1.3-.5-1.2-1.1-1.5-1.1-1.5-.9-.6.1-.6.1-.6 1 .1 1.5 1 1.5 1 .9 1.5 2.3 1.1 2.9.8.1-.6.3-1.1.6-1.4-2.2-.2-4.6-1.1-4.6-4.9 0-1.1.4-2 1-2.7-.1-.3-.4-1.3.1-2.7 0 0 .8-.3 2.7 1a9.4 9.4 0 0 1 5 0c1.9-1.3 2.7-1 2.7-1 .5 1.4.2 2.4.1 2.7.6.7 1 1.6 1 2.7 0 3.8-2.4 4.7-4.6 4.9.3.3.6.9.6 1.9v2.8c0 .3.2.6.7.5A10 10 0 0 0 12 2Z"/></svg>
  Continue with GitHub
</a>
