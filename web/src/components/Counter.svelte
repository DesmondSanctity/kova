<script>
  let { value = 0, duration = 1000, format = (n) => n.toLocaleString() } = $props();
  let display = $state(0);

  $effect(() => {
    const target = Number(value) || 0;
    const start = performance.now();
    let raf;
    function tick(now) {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 3);
      display = target * eased;
      if (t < 1) raf = requestAnimationFrame(tick);
      else display = target;
    }
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });
</script>

<span>{format(Math.round(display))}</span>
