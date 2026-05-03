# React + TypeScript + Vite

This repository is a **full-stack document workflow demo** (working title: **SmartDocs**): a **React 19** UI on **Vite 8**, a **Go** REST API with **Gin** and **GORM**, and **PostgreSQL**. The UI lists documents, shows validation issues and line items, and supports uploads (CSV, TXT, PDF, and optional **image → OCR** on the server).

The stack section below is the standard Vite + React baseline; project-specific setup follows under **Run the application**.

---

## What reviewers get from this repo

| Area | Notes |
|------|--------|
| **Frontend** | TypeScript, React Router, Tailwind CSS; API client layer under `src/api/`. |
| **Backend** | Go module `masterytask`, configurable PostgreSQL, migrations on startup, JSON REST. |
| **Documents** | Upload → parse → persist → list/detail with validation status and issues. |
| **Images** | Optional **OCR.space** integration (API key **only** on the server; never in the browser). |

---

## Prerequisites

- **Node.js** 20+ (for `npm run dev` / `npm run build`)
- **Go** 1.25+ (see `backend/go.mod`)
- **PostgreSQL** 14+ (local or Docker)

---

## Run the application

1. **Database** — create a database and user matching what you will put in `.env`.

2. **Backend** — from `backend/`:
   - Copy [`backend/.env.example`](backend/.env.example) to `backend/.env` and set `DB_*`.
   - Optional: `OCR_SPACE_API_KEY` for PNG/JPEG/WebP uploads ([free OCR.space key](https://ocr.space/ocrapi/freekey)). CSV, TXT, and PDF work without it.
   - Start the API (loads `.env` from the **current working directory**, so run commands **inside `backend/`**):

     ```bash
     cd backend
     go run ./cmd/api
     ```

     Default listen address: `:8080`. Health check: `GET http://localhost:8080/health`.

3. **Frontend** — from the repository root:

   ```bash
   npm install
   npm run dev
   ```

   In development, Vite proxies `/api` to `http://localhost:8080` and rewrites the path (see `vite.config.ts`), so the UI can call the Go API without CORS issues.

---

## Environment variables

**Secrets (database URI, OCR key, host-specific URLs):** define them only in **local `.env` (untracked)** or in your **hosting provider’s environment** (Render, Vercel, etc.). **Never** put real values in this file, in git, or in public issues/PRs.

| Variable | Where | Purpose |
|----------|--------|---------|
| `DATABASE_URL` | local `.env` or API host only | **Either** this **or** the discrete `DB_*` vars. If set, it wins; `DB_*` are ignored. |
| `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`, `DB_SSLMODE`, `DB_TIMEZONE` | same | Required **only when `DATABASE_URL` is unset** (typical local dev). |
| `OCR_SPACE_API_KEY` | API host only | Optional; **image** uploads only. |
| `CORS_ALLOWED_ORIGINS` | API host only | Optional; comma-separated **exact** browser `Origin` values for your deployed SPA. Empty ⇒ no CORS middleware. See [`backend/internal/middleware/cors.go`](backend/internal/middleware/cors.go). |
| `VITE_API_URL` | frontend build env on the host | Only if the SPA calls the API on **another** origin: public API base URL, no trailing slash. Must match how you set CORS. |

Keep **`.env` gitignored**; use [`.env.example`](backend/.env.example) for **names only**, not real credentials.

---

## Scripts (`package.json`)

| Command | Description |
|---------|-------------|
| `npm run dev` | Vite dev server with HMR. |
| `npm run build` | Typecheck + production bundle. |
| `npm run preview` | Preview the production build locally. |
| `npm run lint` | ESLint. |

---

## Supported file formats

The same parsing pipeline backs every upload type — extract text → detect labelled fields → detect a line-item table — so the rules below apply uniformly.

| Format | What works | What breaks gracefully |
|--------|------------|------------------------|
| **CSV** | Document fields and line items via flexible column names: long (`description`, `quantity`, `unit_price`, `line_total`) or short (`desc`, `qty`, `price`, `total`). Document total comes from `grand_total` / `invoice_total` / `document_total` if present, otherwise it is the sum of line totals. | Missing required document fields are flagged as `MISSING_FIELD` issues; status drops to `needs_review`. |
| **TXT** | `Label: value` lines are recognised case-insensitively (incl. `Total Due`, `Grand Total`, `Amount Due`, `Invoice No.`). Numeric values tolerate currency symbols and thousand separators (`$3,960.00`, `1.234,56`, `406 EUR`). A simple table (header row + rows separated by tabs, commas, or 2+ spaces) populates line items. | If only a title and total are present, line items remain empty and the document opens in `needs_review`. |
| **PDF** | Digital PDFs are read page-by-page and converted to a layout-preserving text block (rows joined by 2+ spaces) before going through the TXT parser. | Image-only / scanned PDFs fall back to OCR via OCR.space when `OCR_SPACE_API_KEY` is configured; otherwise a `PDF_NO_EXTRACTABLE_TEXT` issue is added and the document is saved in `needs_review`. |
| **Images (PNG/JPEG/WebP)** | Routed through OCR.space with `isTable=true` and OCR engine 2 to recover aligned columns; the rest of the parser is identical to TXT. | Requires a real OCR.space API key on the server; when text recognition is empty an `IMAGE_OCR_EMPTY` issue is recorded. |

### Status rules

- A document is set to `needs_review` whenever validation produces any issue (missing required field, math mismatch, invalid date, missing currency, etc.).
- `PATCH /documents/:id/status` refuses to set `validated` while required fields are blank or any **unresolved error-severity** issue remains; it returns HTTP 409 with a `blockers` list explaining why.
- Other transitions (`uploaded`, `needs_review`, `rejected`) are unaffected.

---

## Repository layout (high level)

```text
backend/cmd/api/           # API entrypoint
backend/internal/config/   # Env loading (DB, optional OCR, optional CORS list)
backend/internal/middleware/ # Cross-cutting HTTP (CORS)
backend/internal/document/ # Routes, upload, parsing, OCR helper
src/api/                  # Fetch helpers and document endpoints
src/pages/                # Route-level screens
src/components/           # Shared UI (e.g. upload form)
```

---

## Deploy checklist (Vercel + Render + Neon)

**Readiness:** the API now respects the **`PORT`** environment variable (required on Render). For a split front/back deploy, use **one** of the two API strategies below.

### 1) Neon (Postgres)

Create a project in [Neon](https://neon.tech). In the Neon console, open the **connection** / **database URL** (reveal or copy from there only when logged in).

- **Render (API):** add **`DATABASE_URL`** in the service’s **Environment** tab (value comes from Neon, never from the repo).
- **Local:** use **discrete `DB_*`** in `backend/.env` as in [`.env.example`](backend/.env.example), or a local `DATABASE_URL` line in that same private file.

The static frontend does **not** need the database string—only the Go process does.

### 2) Render (Go API)

- **Runtime:** native Go or Docker (Dockerfile optional).
- **Build:** e.g. `go build -o bin/api ./cmd/api` from `backend/` (set root directory to `backend` in Render if the repo is monorepo).
- **Start:** `./bin/api` (or `bin/api` depending on path).
- **Env:** set in the Render dashboard: **`DATABASE_URL`** (or discrete `DB_*`), optional `OCR_SPACE_API_KEY`, optional `CORS_ALLOWED_ORIGINS` (only for cross-origin browser → API), optional `GIN_MODE=release`. Do not paste these into the repository.
- After deploy, note the public **API base URL** from the provider (use it only in Vercel env or `vercel.json`, not in committed docs).

Free-tier Render instances **sleep** when idle; first request after idle can take ~30–60s.

### 3) Vercel (frontend)

**Recommended:** keep the browser on **same origin** and proxy API calls through Vercel (no CORS changes on the Go server).

1. Edit [`vercel.json`](vercel.json): set the rewrite **destination** to your deployed API’s HTTPS origin (placeholder in the file must be replaced **locally** before deploy—do not commit real hosts if your fork is public).
2. Deploy the repo root; **do not** set `VITE_API_URL` in production if you use this rewrite (the app keeps using `/api` like local dev).

How it works: the UI calls `/api/…` on the same host as the SPA; Vercel proxies that path to your API host.

**Alternative (direct API URL):** set **`VITE_API_URL`** at build time to the public API base URL. On the API host, set **`CORS_ALLOWED_ORIGINS`** to the **exact** SPA origins that will call it (comma-separated). For local dev against a remote API, add your local dev origin only in **your own** env, not in the README. Omit **`CORS_ALLOWED_ORIGINS`** when using only the `/api` rewrite (same-origin).

### 4) Smoke test after deploy

- From the browser, hit **`/api/health`** on your deployed frontend URL so the rewrite reaches the API — expect JSON `status` ok.
- Exercise list/upload in the UI as needed.

---

## Vite + React template notes

This template provides a minimal setup for React with Vite, HMR, and ESLint.

Official React plugins:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) (Oxc)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) (SWC)

**React Compiler** is not enabled by default (performance tradeoffs). See the [React Compiler installation guide](https://react.dev/learn/react-compiler/installation).

### Expanding ESLint

For stricter, type-aware rules, replace `tseslint.configs.recommended` with `recommendedTypeChecked` / `strictTypeChecked` and set `parserOptions.project` to your `tsconfig` paths. Optional React-specific plugins: [eslint-plugin-react-x](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-x), [eslint-plugin-react-dom](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-dom). See the generated `eslint.config.js` and the [typescript-eslint docs](https://typescript-eslint.io/getting-started/) for full examples.
