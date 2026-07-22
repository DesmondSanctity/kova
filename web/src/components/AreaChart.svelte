<script>
  import { onMount } from 'svelte';

  let { data = [], labels = [], height = 220, emptyText = 'No checks in the last 14 days' } = $props();

  const W = 640;
  const H = 220;
  const padX = 8;
  const padTop = 18;
  const padBottom = 26;

  let mounted = $state(false);
  let hover = $state(-1);

  const empty = $derived(!data || data.length === 0 || data.every((v) => v === 0));

  let geo = $derived(build(data));

  function build(arr) {
    const pts = arr && arr.length ? arr : new Array(14).fill(0);
    const max = Math.max(...pts, 1);
    const step = pts.length > 1 ? (W - padX * 2) / (pts.length - 1) : 0;
    const y = (v) => padTop + (1 - v / max) * (H - padTop - padBottom);
    const coords = pts.map((v, i) => [padX + i * step, y(v)]);
    const line = coords.map((c, i) => `${i ? 'L' : 'M'}${c[0].toFixed(1)},${c[1].toFixed(1)}`).join(' ');
    const area = `${line} L${coords[coords.length - 1][0].toFixed(1)},${H - padBottom} L${coords[0][0].toFixed(1)},${H - padBottom} Z`;
    const gridY = [0, 0.5, 1].map((f) => padTop + f * (H - padTop - padBottom));
    return { coords, line, area, gridY, max, step };
  }

  onMount(() => {
    const t = setTimeout(() => (mounted = true), 40);
    return () => clearTimeout(t);
  });
</script>

<div class="relative w-full" style="aspect-ratio: {W} / {H}">
  <svg viewBox="0 0 {W} {H}" class="h-full w-full" role="img" aria-label="Checks over time">
    <defs>
      <linearGradient id="chartArea" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="var(--color-violet)" stop-opacity="0.30" />
        <stop offset="100%" stop-color="var(--color-violet)" stop-opacity="0" />
      </linearGradient>
    </defs>

    {#each geo.gridY as gy (gy)}
      <line x1={padX} y1={gy} x2={W - padX} y2={gy} stroke="var(--color-line)" stroke-width="1" stroke-dasharray="2 4" />
    {/each}

    <path d={geo.area} fill="url(#chartArea)" style="opacity:{mounted && !empty ? 1 : 0};transition:opacity .8s ease .2s" />
    <path
      d={geo.line}
      fill="none"
      stroke="var(--color-violet)"
      stroke-width="2.25"
      stroke-linejoin="round"
      stroke-linecap="round"
      pathLength="1"
      style="stroke-dasharray:1;stroke-dashoffset:{mounted && !empty ? 0 : 1};transition:stroke-dashoffset 1.1s cubic-bezier(.4,0,.1,1)"
    />

    {#if !empty}
      {#each geo.coords as c, i (i)}
        <circle
          cx={c[0]}
          cy={c[1]}
          r={hover === i ? 5 : 3}
          fill="var(--color-bg)"
          stroke="var(--color-violet)"
          stroke-width="2"
          style="opacity:{mounted ? (hover === i ? 1 : 0) : 0};transition:opacity .2s,r .15s"
        />
        <rect
          x={c[0] - geo.step / 2}
          y="0"
          width={Math.max(geo.step, 18)}
          height={H}
          fill="transparent"
          role="presentation"
          onmouseenter={() => (hover = i)}
          onmouseleave={() => (hover = -1)}
        />
      {/each}
    {/if}
  </svg>

  {#if empty}
    <div class="absolute inset-0 grid place-items-center">
      <span class="text-[14px] text-faint">{emptyText}</span>
    </div>
  {/if}

  {#if !empty && hover >= 0}
    <div
      class="pointer-events-none absolute -translate-x-1/2 -translate-y-full rounded-lg border border-line bg-surface px-2.5 py-1.5 text-center shadow-xl"
      style="left:{(geo.coords[hover][0] / W) * 100}%;top:{(geo.coords[hover][1] / H) * 100}%"
    >
      <div class="text-[15px] font-semibold text-fg">{data[hover]}</div>
      {#if labels[hover]}<div class="text-[11px] text-faint">{labels[hover]}</div>{/if}
    </div>
  {/if}
</div>
