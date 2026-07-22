<script>
  import { onMount } from 'svelte';

  let banks = $state([]);

  onMount(async () => {
    try {
      const r = await fetch('/v1/banks');
      const d = await r.json();
      banks = (d.banks || [])
        .filter((b) => b.logo && !b.logo.includes('default-image'))
        .slice(0, 24);
    } catch {
      banks = [];
    }
  });
</script>

<div class="grid grid-cols-3 gap-x-6 gap-y-8 sm:grid-cols-4 md:grid-cols-8">
  {#each banks as b, i (i)}
    <div class="flex h-9 items-center justify-center" title={b.name}>
      <img
        src={b.logo}
        alt={b.name}
        class="max-h-8 max-w-[80%] object-contain opacity-85 transition duration-200 hover:opacity-100"
        loading="lazy"
        onerror={(e) => e.currentTarget.closest('div')?.remove()}
      />
    </div>
  {/each}
</div>
