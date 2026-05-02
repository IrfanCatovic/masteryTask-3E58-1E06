import { useEffect, useState } from 'react'
import { fetchDocuments } from '../api/documents'
import { DocumentTable } from '../components/DocumentTable'
import type { Document } from '../types/document'

const FILTERS: { label: string; value: string | undefined }[] = [
  { label: 'Svi', value: undefined },
  { label: 'uploaded', value: 'uploaded' },
  { label: 'needs_review', value: 'needs_review' },
  { label: 'validated', value: 'validated' },
  { label: 'rejected', value: 'rejected' },
]

export function DocumentsPage() {
  const [documents, setDocuments] = useState<Document[]>([])
  const [filter, setFilter] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchDocuments(filter)
      .then((rows) => {
        if (!cancelled) setDocuments(rows)
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Greška pri učitavanju')
          setDocuments([])
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [filter])

  return (
    <section className="mt-10 space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">Dokumenti</h2>
        <div className="flex flex-wrap gap-2">
          {FILTERS.map(({ label, value }) => {
            const active = filter === value
            return (
              <button
                key={label}
                type="button"
                onClick={() => setFilter(value)}
                className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                  active
                    ? 'bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900'
                    : 'border border-neutral-300 bg-white text-neutral-700 hover:bg-neutral-50 dark:border-neutral-600 dark:bg-neutral-900 dark:text-neutral-300 dark:hover:bg-neutral-800'
                }`}
              >
                {label}
              </button>
            )
          })}
        </div>
      </div>

      {loading && (
        <p className="text-sm text-neutral-500 dark:text-neutral-400">Učitavanje…</p>
      )}
      {error && (
        <div
          className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-200"
          role="alert"
        >
          {error}
        </div>
      )}
      {!loading && !error && <DocumentTable documents={documents} />}
    </section>
  )
}
