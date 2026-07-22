// Global toast store (Svelte 5 runes). Shared across islands on a page.
let items = $state([]);
let seq = 0;

function push(type, message, opts = {}) {
 const id = ++seq;
 items.push({ id, type, message });
 const ttl = opts.duration ?? (type === 'error' ? 6000 : 3800);
 if (ttl > 0) setTimeout(() => dismiss(id), ttl);
 return id;
}

function dismiss(id) {
 items = items.filter((t) => t.id !== id);
}

export const toasts = {
 get items() {
  return items;
 },
 dismiss,
};

export const toast = {
 success: (m, o) => push('success', m, o),
 error: (m, o) => push('error', m, o),
 warning: (m, o) => push('warning', m, o),
 info: (m, o) => push('info', m, o),
};
