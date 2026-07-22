<script>
  import { fly, fade } from 'svelte/transition';
  import { toasts } from '../lib/toast.svelte.js';

  const meta = {
    success: {
      ring: 'border-accent/40',
      icon: 'text-accent',
      svg: '<path d="M20 6 9 17l-5-5" stroke-linecap="round" stroke-linejoin="round"/>',
    },
    error: {
      ring: 'border-red-500/45',
      icon: 'text-red-400',
      svg: '<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16.5v.5" stroke-linecap="round"/>',
    },
    warning: {
      ring: 'border-amber-500/45',
      icon: 'text-amber-400',
      svg: '<path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4M12 16.5v.5" stroke-linecap="round"/>',
    },
    info: {
      ring: 'border-violet/45',
      icon: 'text-violet',
      svg: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 7.5v.5" stroke-linecap="round"/>',
    },
  };
</script>

<div class="pointer-events-none fixed right-4 top-4 z-[100] flex w-[min(92vw,400px)] flex-col gap-2.5">
  {#each toasts.items as t (t.id)}
    <div
      in:fly={{ y: -14, duration: 260 }}
      out:fade={{ duration: 160 }}
      class="pointer-events-auto flex items-start gap-3 rounded-xl border {meta[t.type].ring} bg-surface/95 px-4 py-3.5 shadow-2xl backdrop-blur-xl"
    >
      <span class="mt-0.5 shrink-0 {meta[t.type].icon}">
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">{@html meta[t.type].svg}</svg>
      </span>
      <p class="flex-1 text-[15px] leading-snug text-fg">{t.message}</p>
      <button
        onclick={() => toasts.dismiss(t.id)}
        aria-label="Dismiss"
        class="-mr-1 mt-0.5 shrink-0 text-faint transition-colors hover:text-fg"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>
    </div>
  {/each}
</div>
