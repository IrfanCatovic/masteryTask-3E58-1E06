import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { fetchDocument, patchDocumentStatus } from '../api/documents'
import {
  DOCUMENT_STATUSES,
  type Document,
  type DocumentStatus,
  type LineItem,
  type ValidationIssue,
} from '../types/document'

function formatMoney(n: number | undefined, currency?: string): string {
  if (n === undefined || Number.isNaN(n)) return '—'
  const cur = currency?.trim() ? ` ${currency}` : ''
  return `${n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}${cur}`
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return String(iso)
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: '2-digit' })
}

const severityStyle: Record<string, string> = {
  error: 'border-red-200 bg-red-50 text-red-900',
  warning: 'border-amber-200 bg-amber-50 text-amber-900',
}

export function DocumentDetailPage() {
  const { id: idParam } = useParams()
  const navigate = useNavigate()
  const [doc, setDoc] = useState<Document | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusDraft, setStatusDraft] = useState<DocumentStatus>('uploaded')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  const id = idParam ? Number.parseInt(idParam, 10) : NaN

  useEffect(() => {
    if (Number.isNaN(id) || id < 1) {
      setError('Invalid document id')
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)
    setError(null)

    fetchDocument(id)
      .then((d) => {
        if (!cancelled) {
          setDoc(d)
          setStatusDraft(d.status as DocumentStatus)
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Failed to load document')
          setDoc(null)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [id])

  async function handleSaveStatus() {
    if (!doc || Number.isNaN(id)) return
    setSaving(true)
    setSaveError(null)
    try {
      await patchDocumentStatus(id, statusDraft)
      const fresh = await fetchDocument(id)
      setDoc(fresh)
      setStatusDraft(fresh.status as DocumentStatus)
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : 'Failed to update status')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-10 w-40 animate-pulse rounded-xl bg-slate-200" />
        <div className="h-64 animate-pulse rounded-3xl bg-slate-100" />
      </div>
    )
  }

  if (error || !doc) {
    return (
      <div className="rounded-2xl border border-red-200 bg-red-50 px-6 py-8 text-center">
        <p className="font-semibold text-red-900">{error ?? 'Document not found'}</p>
        <Link
          to="/"
          className="mt-4 inline-block text-sm font-semibold text-indigo-600 hover:text-indigo-800"
        >
          Back to list
        </Link>
      </div>
    )
  }

  const lineItems: LineItem[] = doc.line_items ?? []
  const issues: ValidationIssue[] = doc.issues ?? []
  const statusDirty = doc.status !== statusDraft

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Link
          to="/"
          className="inline-flex w-fit items-center gap-2 text-sm font-semibold text-slate-600 hover:text-slate-950"
        >
          <span aria-hidden>←</span> Back to documents
        </Link>
      </div>

      <div className="rounded-3xl border border-white/70 bg-white/95 p-6 shadow-xl shadow-slate-200/60 backdrop-blur sm:p-8">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-600">
              Document
            </p>
            <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950 sm:text-3xl">
              {doc.document_number}
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              #{doc.id} · {doc.document_type} · {doc.supplier_name || '—'}
            </p>
          </div>

          <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-slate-50/80 p-4 sm:min-w-[240px]">
            <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              Workflow status
            </label>
            <select
              value={statusDraft}
              onChange={(e) => setStatusDraft(e.target.value as DocumentStatus)}
              className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-900 shadow-sm"
            >
              {DOCUMENT_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s.replace('_', ' ')}
                </option>
              ))}
            </select>
            <button
              type="button"
              disabled={!statusDirty || saving}
              onClick={() => void handleSaveStatus()}
              className="rounded-xl bg-slate-950 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40"
            >
              {saving ? 'Saving…' : 'Save status'}
            </button>
            {saveError && (
              <p className="text-xs font-medium text-red-700" role="alert">
                {saveError}
              </p>
            )}
          </div>
        </div>

        <dl className="mt-8 grid gap-4 border-t border-slate-100 pt-8 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Total</dt>
            <dd className="mt-1 font-semibold tabular-nums text-slate-950">
              {formatMoney(doc.total, doc.currency)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Issue date</dt>
            <dd className="mt-1 text-slate-800">{formatDate(doc.issue_date)}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Due date</dt>
            <dd className="mt-1 text-slate-800">{formatDate(doc.due_date)}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">Updated</dt>
            <dd className="mt-1 text-slate-600">{formatDate(doc.updated_at)}</dd>
          </div>
        </dl>
      </div>

      {issues.length > 0 && (
        <section className="rounded-3xl border border-white/70 bg-white/95 p-6 shadow-lg backdrop-blur sm:p-8">
          <h2 className="text-lg font-semibold text-slate-950">Validation issues</h2>
          <p className="mt-1 text-sm text-slate-500">
            {issues.length} issue{issues.length === 1 ? '' : 's'} detected for this document.
          </p>
          <ul className="mt-4 space-y-3">
            {issues.map((issue) => (
              <li
                key={issue.id}
                className={`rounded-2xl border px-4 py-3 text-sm ${
                  severityStyle[issue.severity] ?? 'border-slate-200 bg-slate-50 text-slate-800'
                }`}
              >
                <span className="font-mono text-xs font-semibold">{issue.code}</span>
                {issue.field_name && (
                  <span className="ml-2 text-xs text-slate-600">({issue.field_name})</span>
                )}
                <p className="mt-1">{issue.message}</p>
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className="rounded-3xl border border-white/70 bg-white/95 p-6 shadow-lg backdrop-blur sm:p-8">
        <h2 className="text-lg font-semibold text-slate-950">Line items</h2>
        {lineItems.length === 0 ? (
          <p className="mt-4 text-sm text-slate-500">No line items for this document.</p>
        ) : (
          <div className="mt-4 overflow-x-auto rounded-2xl border border-slate-200">
            <table className="w-full min-w-[560px] text-left text-sm">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-3 font-semibold">Description</th>
                  <th className="px-4 py-3 font-semibold">Qty</th>
                  <th className="px-4 py-3 text-right font-semibold">Unit price</th>
                  <th className="px-4 py-3 text-right font-semibold">Line total</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {lineItems.map((row) => (
                  <tr key={row.id} className="hover:bg-slate-50/80">
                    <td className="px-4 py-3 text-slate-900">{row.description}</td>
                    <td className="px-4 py-3 tabular-nums text-slate-700">{row.quantity}</td>
                    <td className="px-4 py-3 text-right tabular-nums text-slate-800">
                      {formatMoney(row.unit_price, doc.currency)}
                    </td>
                    <td className="px-4 py-3 text-right font-medium tabular-nums text-slate-950">
                      {formatMoney(row.line_total, doc.currency)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <div className="flex justify-end">
        <button
          type="button"
          onClick={() => navigate('/')}
          className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50"
        >
          Close
        </button>
      </div>
    </div>
  )
}
