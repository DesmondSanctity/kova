<script>
  let { data = [], stroke = 'var(--color-violet)' } = $props();
  const W = 100;
  const H = 34;
  const pad = 2;
  const uid = 'sp' + Math.random().toString(36).slice(2, 8);

  let paths = $derived(build(data));

  function build(arr) {
    const pts = arr && arr.length ? arr : [0, 0];
    const max = Math.max(...pts, 1);
    const min = Math.min(...pts, 0);
    const span = max - min || 1;
    const step = pts.length > 1 ? (W - pad * 2) / (pts.length - 1) : 0;
    const coords = pts.map((v, i) => [pad + i * step, H - pad - ((v - min) / span) * (H - pad * 2)]);
    const line = coords.map((c, i) => `${i ? 'L' : 'M'}${c[0].toFixed(1)},${c[1].toFixed(1)}`).join(' ');
    const last = coords[coords.length - 1];
    const first = coords[0];
    const area = `${line} L${last[0].toFixed(1)},${H} L${first[0].toFixed(1)},${H} Z`;
    return { line, area };
  }
</script>

<svg viewBox="0 0 100 34" preserveAspectRatio="none" class="h-9 w-full overflow-visible">
  <defs>
    <linearGradient id={uid} x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color={stroke} stop-opacity="0.28" />
      <stop offset="100%" stop-color={stroke} stop-opacity="0" />
    </linearGradient>
  </defs>
  <path d={paths.area} fill="url(#{uid})" />
  <path d={paths.line} fill="none" stroke={stroke} stroke-width="1.6" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
</svg>
