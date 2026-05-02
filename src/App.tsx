import { ApiHealthBadge } from './components/ApiHealthBadge'
import { DocumentsPage } from './pages/DocumentsPage'

function App() {
  return (
    <div className="min-h-dvh bg-[radial-gradient(circle_at_top_left,#dbeafe,transparent_28rem),linear-gradient(180deg,#f8fafc_0%,#eef2ff_45%,#f8fafc_100%)]">
      <header className="border-b border-white/70 bg-white/80 backdrop-blur-xl">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-slate-950 text-sm font-bold text-white shadow-lg shadow-slate-300">
              SD
            </div>
            <div>
              <p className="text-sm font-semibold text-slate-950">SmartDocs</p>
              <p className="hidden text-xs text-slate-500 sm:block">Document processing system</p>
            </div>
          </div>
          <ApiHealthBadge />
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 sm:py-8 lg:px-8">
        <section className="mb-8 overflow-hidden rounded-[2rem] border border-white/70 bg-slate-950 shadow-2xl shadow-slate-300/50">
          <div className="relative px-5 py-8 sm:px-8 lg:px-10">
            <div className="absolute inset-y-0 right-0 hidden w-1/2 bg-[radial-gradient(circle_at_center,#4f46e5,transparent_22rem)] opacity-40 lg:block" />
            <div className="relative max-w-3xl">
              <p className="text-xs font-semibold uppercase tracking-[0.24em] text-indigo-300">
                Operations dashboard
              </p>
              <h1 className="mt-3 text-3xl font-semibold tracking-tight text-white sm:text-4xl lg:text-5xl">
                Process, validate, and review business documents.
              </h1>
              <p className="mt-4 max-w-2xl text-sm leading-6 text-slate-300 sm:text-base">
                Monitor uploaded invoices and purchase orders, catch validation issues, and keep
                the review queue moving from one clean workspace.
              </p>
              <div className="mt-6 flex flex-wrap gap-3 text-xs font-medium text-slate-200">
                <span className="rounded-full border border-white/10 bg-white/10 px-3 py-1.5">
                  CSV/TXT ingestion
                </span>
                <span className="rounded-full border border-white/10 bg-white/10 px-3 py-1.5">
                  Validation issues
                </span>
                <span className="rounded-full border border-white/10 bg-white/10 px-3 py-1.5">
                  PostgreSQL persistence
                </span>
              </div>
            </div>
          </div>
        </section>

        <DocumentsPage />
      </main>
    </div>
  )
}

export default App
