import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  deleteDocument,
  fetchDocument,
  patchDocumentStatus,
  updateDocument,
  type UpdateDocumentPayload,
} from '../api/documents'
import { ConfirmDialog } from '../components/DialogOverlays'
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

// "2025-01-15T00:00:00Z" -> "2025-01-15" so the value works as <input type="date">.
function toDateInput(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString().slice(0, 10)
}

const severityStyle: Record<string, string> = {
  error: 'border-red-200 bg-red-50 text-red-900',
  warning: 'border-amber-200 bg-amber-50 text-amber-900',
}

type EditDraft = {
  document_type: string
  supplier_name: string
  document_number: string
  issue_date: string
  due_date: string
  currency: string
  subtotal: string
  tax_rate: string
  discount_rate: string
  total: string
}

function draftFromDocument(doc: Document): EditDraft {
  return {
    document_type: doc.document_type ?? '',
    supplier_name: doc.supplier_name ?? '',
    document_number: doc.document_number ?? '',
    issue_date: toDateInput(doc.issue_date),
    due_date: toDateInput(doc.due_date),
    currency: doc.currency ?? '',
    subtotal: doc.subtotal != null ? String(doc.subtotal) : '',
    tax_rate: doc.tax_rate != null ? String(doc.tax_rate) : '',
    discount_rate: doc.discount_rate != null ? String(doc.discount_rate) : '',
    total: doc.total != null ? String(doc.total) : '',
  }
}

// Convert the form draft into a PATCH payload, sending all editable fields so the
// server has the full state on which to re-run validation.
function buildPayload(draft: EditDraft): UpdateDocumentPayload {
  const numberOrZero = (s: string) => {
    const n = Number(s)
    return Number.isFinite(n) ? n : 0
  }
  return {
    document_type: draft.document_type.trim(),
    supplier_name: draft.supplier_name.trim(),
    document_number: draft.document_number.trim(),
    issue_date: draft.issue_date,
    due_date: draft.due_date,
    currency: draft.currency.trim(),
    subtotal: numberOrZero(draft.subtotal),
    tax_rate: numberOrZero(draft.tax_rate),
    discount_rate: numberOrZero(draft.discount_rate),
    total: numberOrZero(draft.total),
  }
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

  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<EditDraft | null>(null)
  const [savingDoc, setSavingDoc] = useState(false)
  const [docError, setDocError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

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

  // Map field_name -> issue messages so we can highlight problem fields in the UI.
  const issuesByField = useMemo(() => {
    const map: Record<string, ValidationIssue[]> = {}
    for (const issue of doc?.issues ?? []) {
      const key = issue.field_name || '_other'
      ;(map[key] ??= []).push(issue)
    }
    return map
  }, [doc])

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

  function startEditing() {
    if (!doc) return
    setDraft(draftFromDocument(doc))
    setDocError(null)
    setEditing(true)
  }

  function cancelEditing() {
    setEditing(false)
    setDraft(null)
    setDocError(null)
  }

  async function handleDeleteDocument() {
    if (Number.isNaN(id)) return
    setDeleteError(null)
    setDeleting(true)
    try {
      await deleteDocument(id)
      setDeleteConfirmOpen(false)
      navigate('/')
    } catch (e: unknown) {
      setDeleteConfirmOpen(false)
      setDeleteError(e instanceof Error ? e.message : 'Could not delete document')
    } finally {
      setDeleting(false)
    }
  }

  async function handleSaveDocument() {
    if (!doc || !draft || Number.isNaN(id)) return
    setSavingDoc(true)
    setDocError(null)
    try {
      const updated = await updateDocument(id, buildPayload(draft))
      setDoc(updated)
      setStatusDraft(updated.status as DocumentStatus)
      setEditing(false)
      setDraft(null)
    } catch (e: unknown) {
      setDocError(e instanceof Error ? e.message : 'Failed to save document')
    } finally {
      setSavingDoc(false)
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

  function fieldClass(name: string, base: string) {
    return issuesByField[name]?.length ? `${base} ring-2 ring-red-300` : base
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Link
          to="/"
          className="inline-flex w-fit items-center gap-2 text-sm font-semibold text-slate-600 hover:text-slate-950"
        >
 Back to documents
        </Link>
      </div>

      <div className="rounded-3xl border border-white/70 bg-white/95 p-6 shadow-xl shadow-slate-200/60 backdrop-blur sm:p-8">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-600">
              Document
            </p>
            <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950 sm:text-3xl">
              {doc.document_number || <span className="text-slate-400"> missing number </span>}
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              #{doc.id} · {doc.document_type || 'unknown type'} · {doc.supplier_name || '—'}
            </p>
          </div>

          <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-slate-50/80 p-4 sm:min-w-[240px]">
            <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              Workflow status
            </label>
            <select
              value={statusDraft}
              onChange={(e) => setStatusDraft(e.target.value as DocumentStatus)}
              disabled={editing}
              className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-900 shadow-sm disabled:opacity-50"
            >
              {DOCUMENT_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s.replace('_', ' ')}
                </option>
              ))}
            </select>
            <button
              type="button"
              disabled={!statusDirty || saving || editing}
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

        {editing && draft ? (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void handleSaveDocument()
            }}
            className="mt-8 grid gap-4 border-t border-slate-100 pt-8 sm:grid-cols-2 lg:grid-cols-4"
          >
            <Field label="Document type" issues={issuesByField['document_type']}>
              <input
                value={draft.document_type}
                onChange={(e) => setDraft({ ...draft, document_type: e.target.value })}
                className={fieldClass('document_type', inputClass)}
                placeholder="invoice / purchase_order"
              />
            </Field>
            <Field label="Supplier name" issues={issuesByField['supplier_name']}>
              <input
                value={draft.supplier_name}
                onChange={(e) => setDraft({ ...draft, supplier_name: e.target.value })}
                className={fieldClass('supplier_name', inputClass)}
              />
            </Field>
            <Field label="Document number" issues={issuesByField['document_number']}>
              <input
                value={draft.document_number}
                onChange={(e) => setDraft({ ...draft, document_number: e.target.value })}
                className={fieldClass('document_number', inputClass)}
              />
            </Field>
            <Field label="Currency" issues={issuesByField['currency']}>
              <input
                value={draft.currency}
                onChange={(e) => setDraft({ ...draft, currency: e.target.value })}
                className={fieldClass('currency', inputClass)}
                placeholder="EUR"
              />
            </Field>
            <Field label="Issue date" issues={issuesByField['issue_date']}>
              <input
                type="date"
                value={draft.issue_date}
                onChange={(e) => setDraft({ ...draft, issue_date: e.target.value })}
                className={fieldClass('issue_date', inputClass)}
              />
            </Field>
            <Field label="Due date" issues={issuesByField['due_date']}>
              <input
                type="date"
                value={draft.due_date}
                onChange={(e) => setDraft({ ...draft, due_date: e.target.value })}
                className={fieldClass('due_date', inputClass)}
              />
            </Field>
            <Field label="Subtotal" issues={issuesByField['subtotal']}>
              <input
                type="number"
                step="0.01"
                value={draft.subtotal}
                onChange={(e) => setDraft({ ...draft, subtotal: e.target.value })}
                className={fieldClass('subtotal', inputClass)}
              />
            </Field>
            <Field label="Tax rate %" issues={issuesByField['tax_rate']}>
              <input
                type="number"
                step="0.01"
                value={draft.tax_rate}
                onChange={(e) => setDraft({ ...draft, tax_rate: e.target.value })}
                className={fieldClass('tax_rate', inputClass)}
              />
            </Field>
            <Field label="Discount rate %" issues={issuesByField['discount_rate']}>
              <input
                type="number"
                step="0.01"
                value={draft.discount_rate}
                onChange={(e) => setDraft({ ...draft, discount_rate: e.target.value })}
                className={fieldClass('discount_rate', inputClass)}
              />
            </Field>
            <Field label="Total" issues={issuesByField['total']}>
              <input
                type="number"
                step="0.01"
                value={draft.total}
                onChange={(e) => setDraft({ ...draft, total: e.target.value })}
                className={fieldClass('total', inputClass)}
              />
            </Field>

            <div className="sm:col-span-2 lg:col-span-4 flex flex-wrap items-center justify-end gap-3 pt-2">
              {docError && (
                <p className="mr-auto text-sm font-medium text-red-700" role="alert">
                  {docError}
                </p>
              )}
              <button
                type="button"
                onClick={cancelEditing}
                disabled={savingDoc}
                className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={savingDoc}
                className="rounded-xl bg-indigo-600 px-5 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 disabled:opacity-50"
              >
                {savingDoc ? 'Saving…' : 'Save corrections'}
              </button>
            </div>
          </form>
        ) : (
          <>
            <dl className="mt-8 grid gap-4 border-t border-slate-100 pt-8 sm:grid-cols-2 lg:grid-cols-4">
              <ReadOnlyField label="Total" issues={issuesByField['total']}>
                <span className="font-semibold tabular-nums text-slate-950">
                  {formatMoney(doc.total, doc.currency)}
                </span>
              </ReadOnlyField>
              <ReadOnlyField label="Issue date" issues={issuesByField['issue_date']}>
                {formatDate(doc.issue_date)}
              </ReadOnlyField>
              <ReadOnlyField label="Due date" issues={issuesByField['due_date']}>
                {formatDate(doc.due_date)}
              </ReadOnlyField>
              <ReadOnlyField label="Updated">{formatDate(doc.updated_at)}</ReadOnlyField>
              <ReadOnlyField label="Subtotal" issues={issuesByField['subtotal']}>
                {formatMoney(doc.subtotal, doc.currency)}
              </ReadOnlyField>
              <ReadOnlyField label="Currency" issues={issuesByField['currency']}>
                {doc.currency || '—'}
              </ReadOnlyField>
              <ReadOnlyField label="Tax rate %" issues={issuesByField['tax_rate']}>
                {doc.tax_rate ?? '—'}
              </ReadOnlyField>
              <ReadOnlyField label="Discount rate %" issues={issuesByField['discount_rate']}>
                {doc.discount_rate ?? '—'}
              </ReadOnlyField>
            </dl>

            <div className="mt-6 flex justify-end">
              <button
                type="button"
                onClick={startEditing}
                className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50"
              >
                Edit fields
              </button>
            </div>
          </>
        )}
      </div>

      {issues.length > 0 && (
        <section className="rounded-3xl border border-white/70 bg-white/95 p-6 shadow-lg backdrop-blur sm:p-8">
          <h2 className="text-lg font-semibold text-slate-950">Validation issues</h2>
          <p className="mt-1 text-sm text-slate-500">
            {issues.length} issue{issues.length === 1 ? '' : 's'} detected for this document.
            Use <span className="font-semibold">Edit fields</span> to correct the data — issues
            will be re-evaluated automatically.
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

      {deleteError && (
        <p className="text-sm font-medium text-red-700" role="alert">
          {deleteError}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <button
          type="button"
          disabled={deleting}
          onClick={() => {
            setDeleteError(null)
            setDeleteConfirmOpen(true)
          }}
          className="rounded-xl border border-red-200 bg-white px-4 py-2 text-sm font-semibold text-red-700 shadow-sm hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          Delete document
        </button>
        <button
          type="button"
          onClick={() => navigate('/')}
          className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50"
        >
          Close
        </button>
      </div>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onClose={() => !deleting && setDeleteConfirmOpen(false)}
        onConfirm={() => void handleDeleteDocument()}
        title="Delete this document?"
        description={
          <>
            This permanently removes the document, all line items, and validation issues. This
            action cannot be undone.
          </>
        }
        cancelLabel="Cancel"
        confirmLabel="Delete"
        danger
        loading={deleting}
      />
    </div>
  )
}

const inputClass =
  'w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-indigo-300 focus:outline-none focus:ring-2 focus:ring-indigo-200'

type FieldShellProps = {
  label: string
  issues?: ValidationIssue[]
  children: React.ReactNode
}

function Field({ label, issues, children }: FieldShellProps) {
  return (
    <label className="block">
      <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span>
      <div className="mt-1">{children}</div>
      {issues?.length ? (
        <span className="mt-1 block text-xs font-medium text-red-700">
          {issues.map((i) => i.message).join(' · ')}
        </span>
      ) : null}
    </label>
  )
}

function ReadOnlyField({ label, issues, children }: FieldShellProps) {
  return (
    <div>
      <dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{label}</dt>
      <dd
        className={`mt-1 text-slate-800 ${
          issues?.length ? 'rounded-md bg-red-50 px-2 py-0.5 text-red-900' : ''
        }`}
      >
        {children}
      </dd>
      {issues?.length ? (
        <p className="mt-1 text-xs text-red-700">{issues.map((i) => i.message).join(' · ')}</p>
      ) : null}
    </div>
  )
}
