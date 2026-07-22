<script>
  import { onMount } from 'svelte';

  let banks = $state([]);
  let row1 = $derived(banks.slice(0, 11));
  let row2 = $derived(banks.slice(11, 22));

  onMount(async () => {
    try {
      const r = await fetch('/v1/banks');
      const d = await r.json();
      banks = (d.banks || []).filter((b) => b.logo && !b.logo.includes('default-image'));
    } catch {
      banks = [];
    }
  });
</script>

<div class="space-y-3">
  <div class="marquee">
    <div class="track left">
      {#each [...row1, ...row1] as b, i (i)}
        <div class="tile"><img src={b.logo} alt="" onerror={(e) => e.currentTarget.closest('.tile')?.remove()} /></div>
      {/each}
    </div>
  </div>
  <div class="marquee">
    <div class="track right">
      {#each [...row2, ...row2] as b, i (i)}
        <div class="tile"><img src={b.logo} alt="" onerror={(e) => e.currentTarget.closest('.tile')?.remove()} /></div>
      {/each}
    </div>
  </div>
</div>

<style>
  .marquee {
    overflow: hidden;
    -webkit-mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent);
    mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent);
  }
  .track {
    display: flex;
    gap: 12px;
    width: max-content;
  }
  .track.left {
    animation: scroll-left 36s linear infinite;
  }
  .track.right {
    animation: scroll-right 36s linear infinite;
  }
  .tile {
    width: 58px;
    height: 58px;
    flex: none;
    border-radius: 13px;
    background: #fff;
    border: 1px solid var(--color-line);
    display: grid;
    place-items: center;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.25);
  }
  .tile img {
    max-width: 62%;
    max-height: 58%;
    object-fit: contain;
  }
  @keyframes scroll-left {
    from { transform: translateX(0); }
    to { transform: translateX(-50%); }
  }
  @keyframes scroll-right {
    from { transform: translateX(-50%); }
    to { transform: translateX(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .track { animation: none; }
  }
</style>
