<script>
  import { onMount } from 'svelte';

  let { cols = 12, rows = 6, width = 240, bias = false } = $props();
  const COUNT = cols * rows;
  let cells = $state(Array(COUNT).fill(0));

  function pick() {
    if (bias) {
      // keep pixel-art in the two bottom corners, away from the text columns
      const row = rows - 1 - Math.floor(Math.random() * Math.ceil(rows * 0.5));
      const left = Math.random() < 0.6;
      const col = left
        ? Math.floor(Math.random() * Math.ceil(cols * 0.32))
        : cols - 1 - Math.floor(Math.random() * Math.ceil(cols * 0.22));
      return row * cols + col;
    }
    return Math.floor(Math.random() * COUNT);
  }

  onMount(() => {
    const id = setInterval(() => {
      const n = 1 + Math.floor(Math.random() * 3);
      for (let k = 0; k < n; k++) {
        const i = pick();
        cells[i] = Math.random() > 0.3 ? 2 : 1;
        setTimeout(() => { cells[i] = 0; }, 2400);
      }
    }, 150);
    return () => clearInterval(id);
  });
</script>

<div
  class="grid"
  style={`width:${width}px;grid-template-columns:repeat(${cols},1fr);gap:0;border-top:1px solid rgba(255,255,255,.06);border-left:1px solid rgba(255,255,255,.06)`}
>
  {#each cells as c, i (i)}
    <span
      class="aspect-square transition-colors duration-300"
      style={`border-right:1px solid rgba(255,255,255,.06);border-bottom:1px solid rgba(255,255,255,.06);background:${c === 2 ? '#48008c' : c === 1 ? '#7c3aed' : 'transparent'}`}
    ></span>
  {/each}
</div>
