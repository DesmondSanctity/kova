// Cloudflare Pages middleware: serve the static Astro site, and transparently
// proxy the dynamic backend paths to the Kova Go API (Unikraft).
//
// This keeps everything on one origin, so the dashboard's relative fetches
// (/api, /auth, /v1 …) work unchanged and session cookies stay first-party.
//
// Set the API_BASE variable in the Pages project (Settings → Variables, or the
// [vars] block in wrangler.toml) to your deployed backend, e.g.
//   API_BASE = https://kova.<id>.kraft.host

const PREFIXES = [
 '/api',
 '/auth',
 '/v1',
 '/health',
 '/webhooks',
 '/assets',
 '/r/',
 '/v/',
 '/pay',
];

interface Env {
 API_BASE?: string;
}

export const onRequest: PagesFunction<Env> = async ({ request, env, next }) => {
 const url = new URL(request.url);
 const shouldProxy =
  env.API_BASE &&
  PREFIXES.some((p) => url.pathname === p || url.pathname.startsWith(p));

 if (!shouldProxy) return next();

 const base = env.API_BASE!.replace(/\/$/, '');
 const target = base + url.pathname + url.search;

 // Preserve method, body, and headers; tell the backend the public host/proto
 // so it renders absolute borrower/repayment links on this domain.
 const proxied = new Request(target, request);
 proxied.headers.set('X-Forwarded-Host', url.host);
 proxied.headers.set('X-Forwarded-Proto', url.protocol.replace(':', ''));

 return fetch(proxied);
};
