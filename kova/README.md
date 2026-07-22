# Kova — the credit layer for the credit‑invisible

> **Your bank statement is your credit score.**

Kova turns raw bank/wallet statements into an **explainable credit score** and wraps it in a complete **lend‑and‑collect loop** — score → estimate → apply → disburse → repay — powered by Monnify. Built for the **API Conference Lagos 2026 × Monnify Developer Challenge** (Monnify **sandbox** only).

150M+ Nigerians are locked out of credit because they have no credit history — yet they receive money and pay bills every month. Kova reads that history (statements they can already download), aggregates it across banks, nets out internal transfers, and returns a **score, band, confidence, recommended limit and human‑readable reasons**. A lender then verifies identity, disburses via Monnify, and collects repayment via a Monnify checkout link.

---

## Table of contents

- [Architecture](#architecture)
- [Tech stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Quick start](#quick-start)
- [Using the dashboard — full walkthrough](#using-the-dashboard)
- [Monnify sandbox notes](#monnify-sandbox-notes)
- [Environment variables](#environment-variables)
- [Make targets](#make-targets)
- [HTTP surface](#http-surface)
- [Scoring model](#scoring-model)
- [Testing](#testing)
- [Project layout](#project-layout)
- [Roadmap — coming next](#roadmap)

---

## Architecture

```
Borrower uploads PDFs ─▶ Extract (go-fitz / MuPDF)
                      ─▶ Parse & normalize (bank adapters → canonical transactions)
                      ─▶ Aggregate (merge banks, net internal transfers)
                      ─▶ Features (income, stability, regularity, cashflow, debt, discipline)
                      ─▶ Score (score + band + confidence + limit + reasons)
                      ─▶ Decide (lender rules → approve / counter / auto-decline)
                      ─▶ Monnify (verify account · disburse · collect repayment)
```

Two surfaces sit on top of the same engine:

- **Hosted link flow** — a lender creates a shareable link; the borrower uploads statements on a branded page, gets an estimate, and applies. The lender disburses and collects from the dashboard. **This is the demo path.**
- **Direct API** — `POST /v1/score` for stateless scoring, plus Monnify verify/disburse.

## Tech stack

| Layer    | Tech                                                                                 |
| -------- | ------------------------------------------------------------------------------------ |
| Backend  | **Go 1.25** (`net/http`), embedded static borrower/lender/repay pages                |
| Frontend | **Astro + Svelte 5 (runes) + Tailwind v4** dashboard & marketing site                |
| Database | **PostgreSQL 16** (via Docker), schema auto‑migrated on boot                         |
| Payments | **Monnify sandbox** — account verification, disbursement (with OTP/MFA), collections |
| PDF      | **go‑fitz** (MuPDF) — no system libs required                                        |
| Email    | **Resend** (optional; OTPs/receipts print to the API response when unset)            |

## Prerequisites

- **Go 1.25+**
- **Node 22.12+** (for the dashboard)
- **Docker** (for Postgres)

---

## Quick start

From the repo root (`kova/`):

```bash
# 1. Config — sandbox Monnify keys are already filled in
cp .env.example .env

# 2. Start Postgres (Docker, mapped to host :5433) and the Go API on :8080
make dev            # = docker compose up -d db  +  go run ./cmd/server

# 3. In a second terminal, start the dashboard on :4322
cd web && npm install && npm run dev -- --port 4322
```

Now open **<http://localhost:4322>**.

- The Astro dev server proxies `/api`, `/v1`, `/auth`, `/r`, `/v`, `/pay` to the Go API on `:8080`, so you only ever visit `:4322`.
- The API listens on `:8080` and auto‑creates all tables on first run.
- `KOVA_BASE_URL` in `.env` is `http://localhost:4322` so redirects and payment links resolve to the app — keep the frontend on **4322**.

> Prefer everything in Docker? `make up` builds and runs the API + DB together (you'd still run the dashboard with `npm run dev`).

**Reset the database** for a clean run:

```bash
docker exec -i $(docker ps --filter "ancestor=postgres:16-alpine" -q | head -1) \
  psql -U kova -d kova -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
# then restart the API — migrations recreate every table
```

---

## Using the dashboard

Everything a judge needs to exercise the product, end to end.

### 1. Sign up & onboard

- Go to <http://localhost:4322>, click **Get started**. Sign up with **email + password** (or **Continue with GitHub** if you set the OAuth env vars).
- Onboarding asks for your **organisation name** and **use case**, then drops you on the dashboard.

### 2. Overview

- Portfolio card (total disbursed, a collections progress bar, repaid vs outstanding), checks over time, and disbursed‑over‑time charts.

### 3. Create a borrower link

- **Links → Create link** (add an optional note, e.g. "₦400k working‑capital loan"). You get:
  - a **Borrower link** (`/r/{id}`) to share, and
  - a **Lender view** (`/v/{id}`, score‑only).
- Copy the borrower link and open it in another tab/window.

### 4. Borrower journey (on the branded link page)

1. Enter name, email, requested amount, and pick a **loan product** (if you configured any in Settings).
2. **Upload statements** — 1–3 banks, ~3 months each, **PDF or CSV**. Kova has dedicated parsers for **OPay** and **PalmPay** and a generic balance‑chain parser that handles most Nigerian banks (Kuda, Wema/ALAT, GTBank, etc.). Use any real Nigerian bank/wallet statement.
3. The page shows **"Your score is in"** with an **estimated amount** and a disclaimer that the lender has final say.
4. The borrower clicks **Apply now**, enters **BVN + payout account**, and the account name is verified via Monnify.

### 5. Lender: review, disburse or reject

- Back in the dashboard **Links → open the request**. You'll see the score breakdown, banks analysed, and the decision.
- **Disburse** → Monnify sends an **OTP** (MFA) to the Monnify account email; enter it to authorize the payout. The loan then shows a **repayment timeline** with dates.
- Or **Reject application** → add a **reason**; the borrower is emailed the decline with your reason.
- Applications below your **auto‑decline score threshold** (Settings) are declined automatically.

### 6. Repayments

- **Repayments** lists disbursed loans. Open one to **Send repayment link** (emails the borrower a Monnify checkout link, also auto‑sent on the due date).
- Borrower pays at `/pay/{id}` via Monnify. On the sandbox the checkout **doesn't auto‑redirect** — click **"Back to merchant site"**, or just hit **Check payment** in the dashboard (Kova verifies server‑side with Monnify and marks it repaid).
- Status flows **outstanding → repaid**; the overview stats and portfolio bar update.

### 7. Activity & Settings

- **Activity** — a paginated, human‑readable audit trail (link created, statements scored, disbursed, repaid, etc.). Click any row for details.
- **Settings** —
  - **Branding**: organisation/brand name, **accent colour** and **button text colour** (with a live preview) that themes the borrower/repay pages.
  - **Loan products**: max amount, interest %, tenor days.
  - **Lending rules**: auto‑decline score threshold (default 40).
  - **Support email**: where lender notifications go (falls back to your account email).

### 8. Emails (optional)

Set `RESEND_API_KEY` + a verified `EMAIL_FROM` to send: score/estimate, decline‑with‑reason, lender "ready to review / accepted / repaid", repayment link, and borrower repayment **receipt**. Without a key, OTP codes are returned in the API response so you can still test locally.

---

## Monnify sandbox notes

- **Credentials**: the public sandbox test keys are pre‑filled in `.env.example`. Set `MONNIFY_WALLET_ACCOUNT` to your sandbox wallet source account to disburse.
- **Disbursement is 2‑step (MFA on by default):** `single` transfer → `PENDING_AUTHORIZATION` + emailed OTP → `validate-otp`. Kova handles the OTP prompt + resend in the dashboard.
- **Collections (repayment):** an init transaction returns a `checkoutUrl`; Kova verifies with the Query API using a deterministic `kova_repay_{id}` reference. Sandbox rarely fires the browser redirect, so Kova always verifies server‑side (via the "Check payment" button and the Monnify webhook at `/webhooks/monnify`).

---

## Environment variables

Copy `.env.example` → `.env`. Sandbox defaults work out of the box.

| Variable                                              | Purpose                                                                             |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `DATABASE_URL`                                        | Postgres DSN (default points at the compose DB on `:5433`)                          |
| `KOVA_ADDR`                                           | API listen address (default `:8080`)                                                |
| `KOVA_BASE_URL`                                       | Public origin of the dashboard (`http://localhost:4322`) — used for links/redirects |
| `MONNIFY_BASE_URL`                                    | `https://sandbox.monnify.com`                                                       |
| `MONNIFY_API_KEY` / `MONNIFY_SECRET_KEY`              | Monnify sandbox auth                                                                |
| `MONNIFY_CONTRACT_CODE`                               | Monnify contract code                                                               |
| `MONNIFY_WALLET_ACCOUNT`                              | Source wallet account funds are disbursed from                                      |
| `KOVA_PUBLISHABLE_KEYS` / `KOVA_SECRET_KEYS`          | Optional API keys (comma‑separated) for the direct API                              |
| `KOVA_GITHUB_CLIENT_ID` / `KOVA_GITHUB_CLIENT_SECRET` | Optional "Continue with GitHub"                                                     |
| `RESEND_API_KEY` / `EMAIL_FROM`                       | Optional transactional email (Resend)                                               |

---

## Make targets

```bash
make db        # start Postgres in Docker (host :5433)
make run       # run the API on the host against the compose DB
make dev       # start DB, then the API
make up        # build + run API and DB together in Docker
make down      # stop the Docker stack
make test      # go test ./...
make test-db   # tests against the compose DB
make build     # build the server binary into bin/
```

---

## HTTP surface

**Direct API (key‑authenticated):**

| Method | Path                 | Purpose                                                |
| ------ | -------------------- | ------------------------------------------------------ |
| `GET`  | `/health`            | Liveness                                               |
| `GET`  | `/v1/banks`          | Bank picker data                                       |
| `POST` | `/v1/score`          | Multipart `statements` (1..N PDFs/CSVs) → score report |
| `POST` | `/v1/verify-account` | Monnify account‑name enquiry (secret key)              |
| `POST` | `/v1/disburse`       | Monnify transfer (secret key)                          |
| `POST` | `/v1/requests`       | Create a hosted borrower link                          |

**Hosted link flow (id‑guarded, no key):** `GET /v1/requests/{id}`, `POST /v1/requests/{id}/{intake,score,accept,decline,repay-init}`, and the pages `GET /r/{id}` (borrower), `/v/{id}` (lender), `/pay/{id}` (repay).

**Dashboard API (session):** `/api/me`, `/api/workspace`, `/api/links`, `/api/links/{id}/{disburse,authorize,resend-otp,reject,request-repayment,verify-repayment,resend-offer}`, `/api/audit`.

```bash
# Stateless score
curl -F "statements=@statement.pdf" http://localhost:8080/v1/score

# Verify an account name via Monnify
curl -X POST http://localhost:8080/v1/verify-account \
  -H 'Content-Type: application/json' \
  -d '{"accountNumber":"0123456789","bankCode":"058"}'
```

---

## Scoring model

Score (0–100) is a weighted blend of six components: **income level (25%)**, **income stability (20%)**, **inflow regularity (15%)**, **net cashflow (20%)**, **existing‑debt burden (10%)**, and **financial discipline (10%)**. **Confidence** scales with months of history and transaction density (3 months = floor, 12 = high). Kova scores on **flows, not balances** (wallets that auto‑sweep to zero would otherwise read as broke), and nets **internal transfers** (auto‑save/withdraw, cross‑account self‑transfers) so they never inflate income. Money is handled everywhere as **integer kobo**.

The lender's `decide()` rule turns the score into **approve / counter‑offer / auto‑decline**, honouring the workspace's max amount and auto‑decline threshold.

---

## Testing

```bash
go test ./...
```

Logic packages (parse, aggregate, features, score, pipeline, api, monnify) are cgo‑free and tested against **redacted real‑statement fixtures** and mocked Monnify responses — no live API calls in tests.

---

## Project layout

```
cmd/server                HTTP server entrypoint
cmd/trystmt, cmd/dumptext dev tools: parse / dump a PDF
internal/extract          Extractor interface (+ gofitz MuPDF impl)
internal/parse            canonical model + bank parsers (OPay, PalmPay, generic)
internal/idmatch          name matching (a customer's own accounts)
internal/aggregate        multi-bank merge + internal-transfer netting
internal/features         feature engineering
internal/score            scoring, band, confidence, limit, reasons
internal/pipeline         end-to-end orchestration
internal/monnify          Monnify sandbox client (auth, verify, disburse, collect)
internal/store            Postgres store + migrations
internal/api              HTTP handlers + embedded borrower/lender/repay pages
web/                      Astro + Svelte 5 dashboard & marketing site
docs/                     PRD + build checklist
```

---

## Roadmap

Both are designed but intentionally **out of scope for this submission**:

- **Public API (coming next).** Expose the full lender loop over `/v1` with key‑scoped, paginated reads (`GET /v1/requests`), request‑scoped `disburse / reject / request‑repayment / verify‑repayment`, **outbound webhooks** (workspace callback URL + signing secret for `scored / accepted / disbursed / repaid`), self‑serve **API‑key management**, idempotency keys, per‑key rate limits, and an OpenAPI spec.
- **Drop‑in JS SDK (coming next).** A one‑script‑tag widget (`window.Kova.init(...)`) that embeds the branded statement‑upload + scoring flow into any web app, backed by publishable keys and domain/IP allowlisting.
