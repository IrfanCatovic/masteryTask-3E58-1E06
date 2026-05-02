import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { fetchDocuments } from '../api/documents'
import { DocumentTable } from '../components/DocumentTable'
import { UploadForm } from '../components/UploadForm'
import type { Document } from '../types/document'

const FILTERS: { label: string; value: string | undefined }[] = [
  { label: 'All', value: undefined },
  { label: 'Uploaded', value: 'uploaded' },
  { label: 'Needs review', value: 'needs_review' },
  { label: 'Validated', value: 'validated' },
  { label: 'Rejected', value: 'rejected' },
]

function totalValue(documents: Document[]): number {
  return documents.reduce((sum, doc) => sum + (doc.total ?? 0), 0)
}

export function DocumentsPage() {
  const navigate = useNavigate()
  const [documents, setDocuments] = useState<Document[]>([])
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [duplicateModalOpen, setDuplicateModalOpen] = useState(false)

  const loadDocuments = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const rows = await fetchDocuments(filter)
      setDocuments(rows)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load documents')
      setDocuments([])
    } finally {
      setLoading(false)
    }
  }, [filter])

  useEffect(() => {
    void loadDocuments()
  }, [loadDocuments])

  const stats = useMemo(() => {
    const needsReview = documents.filter((d) => d.status === 'needs_review').length
    const validated = documents.filter((d) => d.status === 'validated').length
    return {
      count: documents.length,
      needsReview,
      validated,
      total: totalValue(documents),
    }
  }, [documents])

  return (
    <section className="space-y-6">
      <div className="grid gap-4 md:grid-cols-3">
        <div className="rounded-2xl border border-white/70 bg-white/85 p-5 shadow-sm shadow-slate-200/70 backdrop-blur">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Documents</p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">{stats.count}</p>
          <p className="mt-1 text-sm text-slate-500">Current filtered result</p>
        </div>
        <div className="rounded-2xl border border-white/70 bg-white/85 p-5 shadow-sm shadow-slate-200/70 backdrop-blur">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Needs review
          </p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-amber-700">
            {stats.needsReview}
          </p>
          <p className="mt-1 text-sm text-slate-500">Validation issues detected</p>
        </div>
        <div className="rounded-2xl border border-white/70 bg-white/85 p-5 shadow-sm shadow-slate-200/70 backdrop-blur">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Visible total
          </p>
          <p className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">
            {stats.total.toLocaleString(undefined, { maximumFractionDigits: 2 })}
          </p>
          <p className="mt-1 text-sm text-slate-500">{stats.validated} validated documents</p>
        </div>
      </div>

      <UploadForm
        onSuccess={(document) => {
          void loadDocuments()
          navigate(`/documents/${document.id}`)
        }}
        onDuplicate={() => setDuplicateModalOpen(true)}
      />

      {duplicateModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4 backdrop-blur-sm"
          role="presentation"
          onClick={() => setDuplicateModalOpen(false)}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="duplicate-modal-title"
            className="max-w-md rounded-3xl border border-slate-200 bg-white p-6 shadow-2xl shadow-slate-900/20"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="duplicate-modal-title" className="text-lg font-semibold text-slate-950">
              Dokument već postoji
            </h3>
            <p className="mt-3 text-sm leading-relaxed text-slate-600">
              Već smo uneli dokument sa tim brojem. Novi unos nije sačuvan ostaje samo postojeći
              zapis.
            </p>
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={() => setDuplicateModalOpen(false)}
                className="rounded-xl bg-slate-950 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-slate-800"
              >
                U redu
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="rounded-3xl border border-white/70 bg-white/90 p-4 shadow-xl shadow-slate-200/60 backdrop-blur sm:p-6">
        <div className="flex flex-col gap-4 border-b border-slate-100 pb-5 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-600">
              Document queue
            </p>
            <h2 className="mt-2 text-xl font-semibold tracking-tight text-slate-950">
              Review and monitor uploaded documents
            </h2>
            <p className="mt-1 max-w-2xl text-sm text-slate-500">
              Filter by workflow status and inspect which files are waiting for validation or
              business review.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            {FILTERS.map(({ label, value }) => {
              const active = filter === value
              return (
                <button
                  key={label}
                  type="button"
                  onClick={() => setFilter(value)}
                  className={`rounded-full px-3.5 py-2 text-sm font-semibold transition ${
                    active
                      ? 'bg-slate-950 text-white shadow-sm'
                      : 'border border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50'
                  }`}
                >
                  {label}
                </button>
              )
            })}
          </div>
        </div>

        <div className="pt-5">
          {loading && (
            <div className="grid gap-3">
              {[0, 1, 2].map((n) => (
                <div key={n} className="h-16 animate-pulse rounded-2xl bg-slate-100" />
              ))}
            </div>
          )}

          {error && (
            <div
              className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800"
              role="alert"
            >
              {error}
            </div>
          )}

          {!loading && !error && <DocumentTable documents={documents} />}
        </div>
      </div>
    </section>
  )
}
