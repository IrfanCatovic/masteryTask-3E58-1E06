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

| Variable | Where | Purpose |
|----------|--------|---------|
| `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`, `DB_SSLMODE`, `DB_TIMEZONE` | `backend/.env` | Required for the API. |
| `OCR_SPACE_API_KEY` | `backend/.env` or host env | Optional; required for **image** uploads only. |
| `VITE_API_URL` | build-time (frontend) | Set when the SPA is served **separately** from the API: full base URL of the API, **no** trailing slash. |

Do **not** commit real secrets; keep `.env` out of version control.

---

## Scripts (`package.json`)

| Command | Description |
|---------|-------------|
| `npm run dev` | Vite dev server with HMR. |
| `npm run build` | Typecheck + production bundle. |
| `npm run preview` | Preview the production build locally. |
| `npm run lint` | ESLint. |

---

## Repository layout (high level)

```text
backend/cmd/api/          # API entrypoint
backend/internal/config/  # Env loading (incl. optional OCR key)
backend/internal/document/ # Routes, upload, parsing, OCR helper
src/api/                  # Fetch helpers and document endpoints
src/pages/                # Route-level screens
src/components/           # Shared UI (e.g. upload form)
```

---

## Deploy checklist

- Provision PostgreSQL and set the same `DB_*` variables on the host.
- Set `OCR_SPACE_API_KEY` on the host only if you need image uploads.
- Build the frontend with `npm run build`; configure **`VITE_API_URL`** if the UI and API have different origins.

---

## Vite + React template notes

This template provides a minimal setup for React with Vite, HMR, and ESLint.

Official React plugins:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) (Oxc)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) (SWC)

**React Compiler** is not enabled by default (performance tradeoffs). See the [React Compiler installation guide](https://react.dev/learn/react-compiler/installation).

### Expanding ESLint

For stricter, type-aware rules, replace `tseslint.configs.recommended` with `recommendedTypeChecked` / `strictTypeChecked` and set `parserOptions.project` to your `tsconfig` paths. Optional React-specific plugins: [eslint-plugin-react-x](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-x), [eslint-plugin-react-dom](https://github.com/Rel1cx/eslint-react/tree/main/packages/plugins/eslint-plugin-react-dom). See the generated `eslint.config.js` and the [typescript-eslint docs](https://typescript-eslint.io/getting-started/) for full examples.
