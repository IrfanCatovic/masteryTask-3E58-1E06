import { useNavigate } from 'react-router-dom'
import type { Document } from '../types/document'

const statusClasses: Record<string, string> = {
  uploaded: 'bg-sky-50 text-sky-700 ring-sky-200',
  needs_review: 'bg-amber-50 text-amber-800 ring-amber-200',
  validated: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  rejected: 'bg-red-50 text-red-700 ring-red-200',
}

function formatMoney(n: number | undefined, currency?: string): string {
  if (n === undefined || Number.isNaN(n)) return '—'
  const cur = currency?.trim() ? ` ${currency}` : ''
  return `${n.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}${cur}`
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
  })
}

function StatusPill({ status }: { status: string }) {
  return (
    <span
      className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ring-1 ring-inset ${
        statusClasses[status] ?? 'bg-slate-100 text-slate-700 ring-slate-200'
      }`}
    >
      {status.replace('_', ' ')}
    </span>
  )
}

type Props = {
  documents: Document[]
}

export function DocumentTable({ documents }: Props) {
  const navigate = useNavigate()

  if (documents.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-6 py-12 text-center shadow-sm">
        <p className="text-sm font-semibold text-slate-900">No documents found</p>
        <p className="mt-1 text-sm text-slate-500">
          Upload a CSV/TXT file or create a document from the API to populate this dashboard.
        </p>
      </div>
    )
  }

  return (
    <>
      <div className="hidden overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm lg:block">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-slate-200 bg-slate-50/80 text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-5 py-3 font-semibold">Document</th>
              <th className="px-5 py-3 font-semibold">Supplier</th>
              <th className="px-5 py-3 font-semibold">Status</th>
              <th className="px-5 py-3 text-right font-semibold">Total</th>
              <th className="px-5 py-3 font-semibold">Issue date</th>
              <th className="px-5 py-3 font-semibold">Updated</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {documents.map((doc) => (
              <tr
                key={doc.id}
                className="cursor-pointer transition hover:bg-slate-50"
                onClick={() => navigate(`/documents/${doc.id}`)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    navigate(`/documents/${doc.id}`)
                  }
                }}
                tabIndex={0}
                role="link"
              >
                <td className="px-5 py-4">
                  <div className="font-semibold text-slate-950">{doc.document_number}</div>
                  <div className="mt-0.5 text-xs text-slate-500">
                    #{doc.id} · {doc.document_type || 'unknown type'}
                  </div>
                </td>
                <td className="max-w-[220px] px-5 py-4">
                  <div className="truncate font-medium text-slate-800" title={doc.supplier_name}>
                    {doc.supplier_name || '—'}
                  </div>
                </td>
                <td className="px-5 py-4">
                  <StatusPill status={doc.status} />
                </td>
                <td className="px-5 py-4 text-right font-semibold tabular-nums text-slate-900">
                  {formatMoney(doc.total, doc.currency)}
                </td>
                <td className="px-5 py-4 text-slate-600">{formatDate(doc.issue_date)}</td>
                <td className="px-5 py-4 text-slate-500">{formatDate(doc.updated_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="grid gap-3 lg:hidden">
        {documents.map((doc) => (
          <article
            key={doc.id}
            className="cursor-pointer rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition hover:border-slate-300 hover:shadow-md"
            onClick={() => navigate(`/documents/${doc.id}`)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                navigate(`/documents/${doc.id}`)
              }
            }}
            tabIndex={0}
            role="link"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-slate-950">
                  {doc.document_number}
                </p>
                <p className="mt-0.5 text-xs text-slate-500">
                  #{doc.id} · {doc.document_type || 'unknown type'}
                </p>
              </div>
              <StatusPill status={doc.status} />
            </div>

            <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
              <div>
                <p className="text-xs font-medium uppercase tracking-wide text-slate-400">
                  Supplier
                </p>
                <p className="mt-1 truncate font-medium text-slate-800">
                  {doc.supplier_name || '—'}
                </p>
              </div>
              <div className="text-right">
                <p className="text-xs font-medium uppercase tracking-wide text-slate-400">Total</p>
                <p className="mt-1 font-semibold tabular-nums text-slate-950">
                  {formatMoney(doc.total, doc.currency)}
                </p>
              </div>
              <div>
                <p className="text-xs font-medium uppercase tracking-wide text-slate-400">Issue</p>
                <p className="mt-1 text-slate-700">{formatDate(doc.issue_date)}</p>
              </div>
              <div className="text-right">
                <p className="text-xs font-medium uppercase tracking-wide text-slate-400">
                  Updated
                </p>
                <p className="mt-1 text-slate-700">{formatDate(doc.updated_at)}</p>
              </div>
            </div>
          </article>
        ))}
      </div>
    </>
  )
}
