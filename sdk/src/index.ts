/**
 * Kova SDK — drop-in credit scoring widget + client for any JS/TS frontend.
 *
 *   import { Kova } from "@kova/sdk";
 *   Kova.init({ key: "pk_live_...", apiBase: "https://api.kova.dev" }).open();
 *
 * Link mode (individual lenders): mount against a request the borrower opens.
 *   Kova.init({ requestId: "abc", container: el, autoOpen: true });
 */

export interface Reason {
 factor: string;
 impact: 'positive' | 'negative';
 detail: string;
}

export interface ScoreFeatures {
 monthsCovered: number;
 transactionCount: number;
 totalInflow: number;
 netCashflow: number;
 avgMonthlyInflow: number;
 inflowRegularity: number;
 distinctIncomeSources: number;
 debtRatio: number;
}

export interface ScoreResult {
 score: number;
 band: string;
 confidence: number;
 limitRecommendation: number;
 reasons: Reason[];
 features: ScoreFeatures;
}

export interface ScoreReport {
 files: {
  filename: string;
  bank?: string;
  parsed: boolean;
  transactions: number;
  error?: string;
 }[];
 accounts: string[];
 banks: string[];
 score: ScoreResult;
}

export interface Bank {
 name: string;
 slug: string;
 code: string;
 logo?: string;
 supported?: boolean;
}

export interface KovaOptions {
 /** Publishable key (pk_...). Required unless `requestId` is set. */
 key?: string;
 /** Assessment request id (link mode). No key needed — the link is the capability. */
 requestId?: string;
 /** Base URL of the Kova API. Defaults to the current origin. */
 apiBase?: string;
 /** Mount inline into this element instead of a modal overlay. */
 container?: HTMLElement;
 theme?: 'dark' | 'light';
 autoOpen?: boolean;
 onResult?: (report: ScoreReport) => void;
 /** Fired in link mode when statements are received (scoring happens async; the result is emailed). */
 onSubmitted?: () => void;
 onError?: (error: Error) => void;
}

export interface KovaInstance {
 open(): void;
 close(): void;
 destroy(): void;
}

const FALLBACK_BANKS: Bank[] = [
 {
  name: 'OPay',
  slug: 'paycom',
  code: '999992',
  logo: 'https://nigerianbanks.xyz/logo/paycom.png',
  supported: true,
 },
 {
  name: 'Access Bank',
  slug: 'access-bank',
  code: '044',
  logo: 'https://nigerianbanks.xyz/logo/access-bank.png',
 },
 {
  name: 'Guaranty Trust Bank',
  slug: 'guaranty-trust-bank',
  code: '058',
  logo: 'https://nigerianbanks.xyz/logo/guaranty-trust-bank.png',
 },
 {
  name: 'Kuda Bank',
  slug: 'kuda-bank',
  code: '50211',
  logo: 'https://nigerianbanks.xyz/logo/kuda-bank.png',
 },
];

const BAND_COLOR: Record<string, string> = {
 A: '#22c55e',
 B: '#38bdf8',
 C: '#f59e0b',
 D: '#fb923c',
 E: '#ef4444',
};

interface Entry {
 bank: Bank;
 file: File | null;
}

const naira = (n: number) => '₦' + Math.round(n).toLocaleString('en-NG');

function h<K extends keyof HTMLElementTagNameMap>(
 tag: K,
 props: Partial<HTMLElementTagNameMap[K]> & {
  class?: string;
  html?: string;
 } = {},
 kids: (Node | string)[] = [],
): HTMLElementTagNameMap[K] {
 const el = document.createElement(tag);
 const { class: cls, html, ...rest } = props as any;
 if (cls) el.className = cls;
 if (html !== undefined) el.innerHTML = html;
 Object.assign(el, rest);
 for (const c of kids)
  el.append(c instanceof Node ? c : document.createTextNode(c));
 return el;
}

class Widget implements KovaInstance {
 private opts: KovaOptions & { apiBase: string; theme: 'dark' | 'light' };
 private host: HTMLDivElement;
 private root: ShadowRoot;
 private banks: Bank[] = FALLBACK_BANKS;
 private maxBanks = 3;
 private entries: Entry[] = [];
 private mounted = false;

 constructor(opts: KovaOptions) {
  if (!opts.key && !opts.requestId)
   throw new Error('Kova: `key` or `requestId` is required');
  this.opts = {
   ...opts,
   apiBase: (opts.apiBase || location.origin).replace(/\/$/, ''),
   theme: opts.theme || 'dark',
  };
  this.host = document.createElement('div');
  this.host.setAttribute('data-kova', '');
  this.root = this.host.attachShadow({ mode: 'open' });
  this.entries = [{ bank: FALLBACK_BANKS[0], file: null }];
  this.loadBanks();
 }

 private async loadBanks() {
  try {
   const res = await fetch(`${this.opts.apiBase}/v1/banks`);
   if (res.ok) {
    const data = await res.json();
    if (Array.isArray(data.banks) && data.banks.length) this.banks = data.banks;
    if (data.maxBanks) this.maxBanks = data.maxBanks;
    this.entries = this.entries.map((e) => ({ ...e, bank: this.banks[0] }));
   }
  } catch {
   /* keep fallback */
  }
  if (this.mounted) this.renderUpload();
 }

 open() {
  if (this.mounted) return;
  this.root.innerHTML = `<style>${styles(this.opts.theme)}</style>`;
  const inline = !!this.opts.container;
  const shell = h('div', { class: inline ? 'kd-inline' : 'kd-overlay' });
  if (!inline)
   shell.addEventListener('click', (e) => {
    if (e.target === shell) this.close();
   });
  shell.append(h('div', { class: 'kd-card' }));
  this.root.append(shell);
  (this.opts.container || document.body).append(this.host);
  this.mounted = true;
  this.renderUpload();
 }

 close() {
  if (this.mounted) {
   this.host.remove();
   this.mounted = false;
  }
 }
 destroy() {
  this.close();
 }

 private card() {
  return this.root.querySelector('.kd-card') as HTMLElement;
 }

 private header(sub: string) {
  const wrap = h('div', { class: 'kd-head' });
  const row = h('div', { class: 'kd-head-row' }, [
   h('div', { class: 'kd-brand' }, [h('span', { class: 'kd-dot' }), 'Kova']),
  ]);
  if (!this.opts.container) {
   const x = h('button', {
    class: 'kd-x',
    html: '&times;',
    ariaLabel: 'Close',
   });
   x.addEventListener('click', () => this.close());
   row.append(x);
  }
  wrap.append(
   row,
   h('div', { class: 'kd-title' }, ['Verify your income']),
   h('div', { class: 'kd-sub' }, [sub]),
  );
  return wrap;
 }

 private renderUpload() {
  const card = this.card();
  card.innerHTML = '';
  card.append(
   this.header(
    `Add up to ${this.maxBanks} bank statements — 3 months each. PDF only.`,
   ),
  );

  const list = h('div', { class: 'kd-list' });
  this.entries.forEach((entry, i) => list.append(this.entryRow(entry, i)));
  card.append(list);

  const add = h('button', { class: 'kd-add' }, ['+ Add another bank']);
  add.disabled = this.entries.length >= this.maxBanks;
  add.addEventListener('click', () => {
   if (this.entries.length < this.maxBanks) {
    this.entries.push({ bank: this.banks[0], file: null });
    this.renderUpload();
   }
  });
  card.append(add);

  const submit = h('button', { class: 'kd-submit' }, ['Get score']);
  submit.disabled = !this.entries.some((e) => e.file);
  submit.addEventListener('click', () => this.submit());
  card.append(submit);
  card.append(
   h('div', { class: 'kd-legal' }, [
    this.opts.requestId
     ? 'Your statements are analyzed privately and only the score is shared.'
     : 'Your statements are analyzed to estimate creditworthiness.',
   ]),
  );
 }

 private entryRow(entry: Entry, index: number) {
  const row = h('div', { class: 'kd-row' });
  row.append(this.bankPicker(entry));

  const fileInput = h('input', {
   class: 'kd-file',
   type: 'file',
   accept: 'application/pdf',
  });
  const drop = h('label', { class: 'kd-drop' }, [
   h('span', { class: 'kd-drop-text' }, [
    entry.file ? entry.file.name : 'Upload statement',
   ]),
  ]);
  drop.append(fileInput);
  fileInput.addEventListener('change', () => {
   entry.file = fileInput.files?.[0] || null;
   (drop.querySelector('.kd-drop-text') as HTMLElement).textContent = entry.file
    ? entry.file.name
    : 'Upload statement';
   drop.classList.toggle('kd-has', !!entry.file);
   this.refreshSubmit();
  });
  row.append(drop);

  if (this.entries.length > 1) {
   const rm = h('button', {
    class: 'kd-remove',
    html: '&times;',
    ariaLabel: 'Remove',
   });
   rm.addEventListener('click', () => {
    this.entries.splice(index, 1);
    this.renderUpload();
   });
   row.append(rm);
  }
  return row;
 }

 private bankPicker(entry: Entry) {
  const wrap = h('div', { class: 'kd-picker' });
  const btn = h('button', { class: 'kd-picker-btn', type: 'button' });
  const render = () => {
   btn.innerHTML = '';
   btn.append(
    bankChip(entry.bank),
    h('span', { class: 'kd-caret', html: '▾' }),
   );
  };
  render();

  const menu = h('div', { class: 'kd-menu' });
  const search = h('input', {
   class: 'kd-search',
   placeholder: 'Search banks',
   type: 'text',
  }) as HTMLInputElement;
  const listEl = h('div', { class: 'kd-menu-list' });
  const paint = (q = '') => {
   listEl.innerHTML = '';
   for (const b of this.banks.filter((b) =>
    b.name.toLowerCase().includes(q.toLowerCase()),
   )) {
    const opt = h('button', { class: 'kd-opt', type: 'button' }, [bankChip(b)]);
    opt.addEventListener('click', () => {
     entry.bank = b;
     render();
     menu.classList.remove('kd-open');
    });
    listEl.append(opt);
   }
  };
  search.addEventListener('input', () => paint(search.value));
  menu.append(search, listEl);

  btn.addEventListener('click', (e) => {
   e.preventDefault();
   const open = menu.classList.toggle('kd-open');
   if (open) {
    paint();
    search.value = '';
    search.focus();
   }
  });
  wrap.append(btn, menu);
  return wrap;
 }

 private refreshSubmit() {
  const submit = this.card().querySelector(
   '.kd-submit',
  ) as HTMLButtonElement | null;
  if (submit) submit.disabled = !this.entries.some((e) => e.file);
 }

 private async submit() {
  const files = this.entries.filter((e) => e.file);
  if (!files.length) return;
  this.renderLoading();
  const fd = new FormData();
  for (const e of files) {
   fd.append(
    'statements',
    new File([e.file!], `${e.bank.slug}_${e.file!.name}`, {
     type: 'application/pdf',
    }),
   );
  }
  const url = this.opts.requestId
   ? `${this.opts.apiBase}/v1/requests/${this.opts.requestId}/score`
   : `${this.opts.apiBase}/v1/score`;
  const headers: Record<string, string> = {};
  if (this.opts.key) headers.Authorization = `Bearer ${this.opts.key}`;
  try {
   const res = await fetch(url, { method: 'POST', headers, body: fd });
   const data = await res.json();
   if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
   // Link mode scores asynchronously and emails the result — no report returned.
   if (this.opts.requestId && (data.status === 'received' || !data.score)) {
    this.opts.onSubmitted?.();
    this.renderSubmitted();
    return;
   }
   this.opts.onResult?.(data as ScoreReport);
   this.renderResult(data as ScoreReport);
  } catch (err) {
   const e = err instanceof Error ? err : new Error(String(err));
   this.opts.onError?.(e);
   this.renderError(e.message);
  }
 }

 private renderLoading() {
  const card = this.card();
  card.innerHTML = '';
  card.append(
   this.header('Analyzing your statements…'),
   h('div', { class: 'kd-loading' }, [h('div', { class: 'kd-spinner' })]),
  );
 }

 private renderSubmitted() {
  const card = this.card();
  card.innerHTML = '';
  card.append(
   this.header('Statements received'),
   h('div', { class: 'kd-legal' }, [
    'Your statements are being analysed. You will receive your result by email shortly.',
   ]),
  );
 }

 private renderError(message: string) {
  const card = this.card();
  card.innerHTML = '';
  card.append(
   this.header('Something went wrong'),
   h('div', { class: 'kd-error' }, [message]),
  );
  const back = h('button', { class: 'kd-submit' }, ['Try again']);
  back.addEventListener('click', () => this.renderUpload());
  card.append(back);
 }

 private renderResult(report: ScoreReport) {
  const card = this.card();
  card.innerHTML = '';
  const s = report.score,
   f = s.features,
   color = BAND_COLOR[s.band] || '#9b9ba4';
  card.append(
   this.header(
    this.opts.requestId
     ? 'Result shared with the lender'
     : `Assessed from ${report.banks.join(', ') || 'your statements'}`,
   ),
  );

  const c = 2 * Math.PI * 42;
  const dial = h('div', { class: 'kd-dial' });
  dial.innerHTML = `
      <svg class="kd-ring" viewBox="0 0 100 100" width="112" height="112">
        <circle cx="50" cy="50" r="42" fill="none" stroke="var(--kd-track)" stroke-width="7"/>
        <circle cx="50" cy="50" r="42" fill="none" stroke="${color}" stroke-width="7" stroke-linecap="round"
          stroke-dasharray="${c}" stroke-dashoffset="${c * (1 - s.score / 100)}" transform="rotate(-90 50 50)"/>
      </svg><div class="kd-dial-num">${s.score}<span>band ${s.band}</span></div>`;

  const meta = h('div', { class: 'kd-meta' }, [
   metaItem('Confidence', `${Math.round(s.confidence * 100)}%`),
   metaItem('Limit', naira(s.limitRecommendation)),
   metaItem('Avg monthly', naira(f.avgMonthlyInflow)),
  ]);
  card.append(h('div', { class: 'kd-result-top' }, [dial, meta]));

  const reasons = h('ul', { class: 'kd-reasons' });
  for (const r of s.reasons.slice(0, 5)) {
   reasons.append(
    h('li', { class: r.impact === 'positive' ? 'kd-pos' : 'kd-neg' }, [
     h('span', { class: 'kd-pill' }),
     h('span', {}, [r.detail]),
    ]),
   );
  }
  card.append(reasons);

  if (!this.opts.requestId) {
   const again = h('button', { class: 'kd-add' }, ['Start over']);
   again.addEventListener('click', () => {
    this.entries = [{ bank: this.banks[0], file: null }];
    this.renderUpload();
   });
   card.append(again);
  }
 }
}

function bankChip(b: Bank): HTMLElement {
 const chip = h('span', { class: 'kd-chip' });
 const img = h('img', { class: 'kd-logo', src: b.logo || '', alt: '' });
 img.addEventListener('error', () => {
  img.style.display = 'none';
 });
 chip.append(img, h('span', { class: 'kd-chip-name' }, [b.name]));
 return chip;
}

function metaItem(label: string, value: string): HTMLElement {
 return h('div', { class: 'kd-meta-item' }, [
  h('span', { class: 'kd-meta-label' }, [label]),
  h('b', {}, [value]),
 ]);
}

function styles(theme: 'dark' | 'light'): string {
 const dark = theme === 'dark';
 const t = dark
  ? {
     bg: '#0b0b0d',
     panel: '#141417',
     line: '#26262b',
     ink: '#f4f4f5',
     muted: '#9b9ba4',
     track: '#26262b',
     field: '#161619',
    }
  : {
     bg: '#ffffff',
     panel: '#f6f6f7',
     line: '#e5e5ea',
     ink: '#0b0b0d',
     muted: '#6b6b74',
     track: '#e5e5ea',
     field: '#fafafa',
    };
 return `
  :host { all: initial; }
  * { box-sizing: border-box; font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
  .kd-overlay { position: fixed; inset: 0; background: rgba(6,6,8,.72); backdrop-filter: blur(6px); display: grid; place-items: center; z-index: 2147483000; padding: 20px; animation: kd-fade .18s ease; }
  .kd-inline { display: block; }
  @keyframes kd-fade { from { opacity: 0 } to { opacity: 1 } }
  .kd-card { --kd-track:${t.track}; width: 100%; max-width: 540px; background: ${t.bg}; color: ${t.ink};
    border: 1px solid ${t.line}; border-radius: 18px; padding: 26px; box-shadow: 0 24px 70px rgba(0,0,0,.5); }
  .kd-inline .kd-card { box-shadow: none; max-width: 560px; margin: 0 auto; }
  .kd-head-row { display: flex; align-items: center; justify-content: space-between; }
  .kd-brand { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 14px; }
  .kd-dot { width: 9px; height: 9px; border-radius: 50%; background: #22c55e; }
  .kd-x, .kd-remove { background: transparent; border: 0; color: ${t.muted}; font-size: 22px; line-height: 1; cursor: pointer; padding: 2px 6px; border-radius: 8px; }
  .kd-x:hover, .kd-remove:hover { color: ${t.ink}; background: ${t.panel}; }
  .kd-title { font-size: 20px; font-weight: 600; margin-top: 14px; letter-spacing: -.3px; }
  .kd-sub { font-size: 13px; color: ${t.muted}; margin-top: 4px; }
  .kd-list { margin-top: 18px; display: flex; flex-direction: column; gap: 10px; }
  .kd-row { display: flex; gap: 8px; align-items: stretch; }
  .kd-picker { position: relative; flex: 0 0 44%; }
  .kd-picker-btn { width: 100%; height: 44px; display: flex; align-items: center; justify-content: space-between; gap: 6px;
    background: ${t.field}; color: ${t.ink}; border: 1px solid ${t.line}; border-radius: 10px; padding: 0 10px; cursor: pointer; }
  .kd-caret { color: ${t.muted}; font-size: 11px; }
  .kd-chip { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .kd-logo { width: 18px; height: 18px; border-radius: 5px; object-fit: contain; background: #fff; }
  .kd-chip-name { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .kd-menu { position: absolute; top: 48px; left: 0; right: 0; z-index: 5; display: none; flex-direction: column;
    background: ${t.bg}; border: 1px solid ${t.line}; border-radius: 12px; padding: 8px; box-shadow: 0 16px 40px rgba(0,0,0,.5); }
  .kd-menu.kd-open { display: flex; }
  .kd-search { height: 36px; border: 1px solid ${t.line}; background: ${t.field}; color: ${t.ink}; border-radius: 8px; padding: 0 10px; font-size: 13px; margin-bottom: 6px; }
  .kd-menu-list { max-height: 240px; overflow: auto; display: flex; flex-direction: column; }
  .kd-opt { display: flex; align-items: center; justify-content: space-between; gap: 8px; background: transparent; border: 0; color: ${t.ink};
    padding: 8px; border-radius: 8px; cursor: pointer; text-align: left; }
  .kd-opt:hover { background: ${t.panel}; }
  .kd-drop { flex: 1; position: relative; display: flex; align-items: center; padding: 0 14px; height: 44px;
    background: ${t.field}; border: 1px dashed ${t.line}; border-radius: 10px; cursor: pointer; overflow: hidden; }
  .kd-drop.kd-has { border-style: solid; }
  .kd-drop-text { font-size: 13px; color: ${t.muted}; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .kd-drop.kd-has .kd-drop-text { color: ${t.ink}; }
  .kd-file { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .kd-add { margin-top: 12px; width: 100%; background: transparent; color: ${t.ink}; border: 1px solid ${t.line}; border-radius: 10px; height: 42px; font-size: 13px; cursor: pointer; }
  .kd-add:hover:not(:disabled) { background: ${t.panel}; }
  .kd-add:disabled { opacity: .4; cursor: default; }
  .kd-submit { margin-top: 12px; width: 100%; height: 46px; border: 0; border-radius: 11px; cursor: pointer; font-size: 14px; font-weight: 600; background: ${t.ink}; color: ${t.bg}; transition: opacity .15s; }
  .kd-submit:hover:not(:disabled) { opacity: .9; }
  .kd-submit:disabled { opacity: .35; cursor: default; }
  .kd-legal { margin-top: 12px; font-size: 11px; color: ${t.muted}; text-align: center; }
  .kd-loading { display: grid; place-items: center; padding: 48px 0; }
  .kd-spinner { width: 34px; height: 34px; border-radius: 50%; border: 3px solid ${t.line}; border-top-color: ${t.ink}; animation: kd-spin .8s linear infinite; }
  @keyframes kd-spin { to { transform: rotate(360deg); } }
  .kd-error { margin-top: 16px; padding: 12px 14px; border-radius: 10px; background: rgba(239,68,68,.12); color: #fca5a5; font-size: 13px; }
  .kd-result-top { display: flex; align-items: center; gap: 18px; margin-top: 18px; }
  .kd-dial { position: relative; width: 112px; height: 112px; flex: 0 0 auto; }
  .kd-dial-num { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; font-size: 32px; font-weight: 700; }
  .kd-dial-num span { font-size: 11px; font-weight: 500; color: ${t.muted}; margin-top: 2px; }
  .kd-meta { display: flex; flex-direction: column; gap: 10px; }
  .kd-meta-item { display: flex; flex-direction: column; }
  .kd-meta-label { font-size: 11px; color: ${t.muted}; }
  .kd-meta-item b { font-size: 16px; }
  .kd-reasons { list-style: none; padding: 0; margin: 18px 0 0; }
  .kd-reasons li { display: flex; gap: 10px; align-items: flex-start; padding: 9px 0; border-top: 1px solid ${t.line}; font-size: 13px; }
  .kd-pill { flex: 0 0 auto; width: 6px; height: 18px; border-radius: 6px; margin-top: 1px; }
  .kd-pos .kd-pill { background: #22c55e; } .kd-neg .kd-pill { background: #ef4444; }
  `;
}

export interface KovaClientOptions {
 /** Base URL of the Kova API. Defaults to the current origin. */
 apiBase?: string;
 /** Secret key (sk_...) — required for creating links, listing and rejecting. */
 key: string;
}

/**
 * Programmatic client for creditworthiness assessment: create borrower links,
 * read scored results, and reject offers. Disbursement is a first-party
 * dashboard action (it moves money from your connected Monnify wallet) and is
 * intentionally not part of the API/SDK.
 */
export class KovaClient {
 private base: string;
 private key: string;
 constructor(options: KovaClientOptions) {
  this.base = (
   options.apiBase || (typeof location !== 'undefined' ? location.origin : '')
  ).replace(/\/$/, '');
  this.key = options.key;
 }
 private async req(path: string, method = 'GET', body?: unknown): Promise<any> {
  const headers: Record<string, string> = {
   Authorization: `Bearer ${this.key}`,
  };
  if (body) headers['Content-Type'] = 'application/json';
  const res = await fetch(this.base + path, {
   method,
   headers,
   body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
 }
 /** Create a shareable borrower link. */
 createLink(
  note = '',
 ): Promise<{ id: string; borrowerUrl: string; viewUrl: string }> {
  return this.req('/api/links', 'POST', { note });
 }
 /** List all links/requests for the workspace. */
 listLinks(): Promise<{ links: any[] }> {
  return this.req('/api/links');
 }
 /** Read a single request (status, decision, offer, score report). */
 getRequest(id: string): Promise<any> {
  return this.req(`/v1/requests/${id}`);
 }
 /** Reject a scored/accepted request before payout. */
 reject(id: string): Promise<{ status: string }> {
  return this.req(`/api/links/${id}/reject`, 'POST');
 }
}

export const Kova = {
 init(options: KovaOptions): KovaInstance {
  const w = new Widget(options);
  if (options.autoOpen) w.open();
  return w;
 },
 /** Server/programmatic client for creditworthiness assessment (use a secret key). */
 client(options: KovaClientOptions): KovaClient {
  return new KovaClient(options);
 },
};

export default Kova;
