<script>
  import { onMount } from 'svelte';

  let banks = $state([]);
  // Outer-edge orbit only (left/right rails + top/bottom) so the centered card
  // keeps a clear safe zone. Square app-icon tiles, Render-style.
  const pos = [
    [10, 7], [40, 5], [66, 8], [84, 14],
    [8, 83], [36, 86], [64, 85], [83, 79],
    [7, 31], [7, 59], [88, 37], [88, 61],
    [24, 80], [72, 11],
  ];

  onMount(async () => {
    try {
      const r = await fetch('/v1/banks');
      const d = await r.json();
      banks = (d.banks || [])
        .filter((b) => b.logo && !b.logo.includes('default-image'))
        .slice(0, pos.length);
    } catch {
      banks = [];
    }
  });
</script>

<div class="pointer-events-none absolute inset-0">
  {#each banks as b, i (i)}
    <div
      class="fl absolute"
      style={`top:${pos[i][0]}%;left:${pos[i][1]}%;animation-delay:${(i * 0.4).toFixed(1)}s`}
    >
      <img src={b.logo} alt="" onerror={(e) => e.currentTarget.closest('.fl')?.remove()} />
    </div>
  {/each}
</div>

<style>
  .fl {
    width: 54px;
    height: 54px;
    border-radius: 12px;
    background: #fff;
    display: grid;
    place-items: center;
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.5);
    animation: bob 6s ease-in-out infinite;
  }
  .fl img {
    max-width: 60%;
    max-height: 56%;
    object-fit: contain;
  }
  @keyframes bob {
    50% {
      transform: translateY(-11px);
    }
  }
  @media (max-width: 860px) {
    .fl {
      display: none;
    }
  }
</style>
