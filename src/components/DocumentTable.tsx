import type { Document } from '../types/document'

function formatMoney(n: number | undefined, currency?: string): string {
  if (n === undefined || Number.isNaN(n)) return '—'
  const cur = currency?.trim() ? ` ${currency}` : ''
  return `${n.toFixed(2)}${cur}`
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString()
}

type Props = {
  documents: Document[]
}

export function DocumentTable({ documents }: Props) {
  if (documents.length === 0) {
    return (
      <p className="rounded-lg border border-neutral-200 bg-neutral-50 px-4 py-8 text-center text-sm text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/30 dark:text-neutral-400">
        Nema dokumenata za prikaz.
      </p>
    )
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-neutral-200 dark:border-neutral-700">
      <table className="w-full min-w-[640px] text-left text-sm">
        <thead className="border-b border-neutral-200 bg-neutral-50 dark:border-neutral-700 dark:bg-neutral-900/50">
          <tr>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">ID</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Broj</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Tip</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Dobavljač</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Status</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Total</th>
            <th className="px-3 py-2 font-medium text-neutral-700 dark:text-neutral-300">Issue</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-neutral-200 dark:divide-neutral-700">
          {documents.map((d) => (
            <tr key={d.id} className="bg-white hover:bg-neutral-50 dark:bg-neutral-950 dark:hover:bg-neutral-900/80">
              <td className="whitespace-nowrap px-3 py-2 font-mono text-neutral-600 dark:text-neutral-400">
                {d.id}
              </td>
              <td className="px-3 py-2 text-neutral-900 dark:text-neutral-100">{d.document_number}</td>
              <td className="px-3 py-2 text-neutral-700 dark:text-neutral-300">{d.document_type}</td>
              <td className="max-w-[180px] truncate px-3 py-2 text-neutral-700 dark:text-neutral-300" title={d.supplier_name}>
                {d.supplier_name}
              </td>
              <td className="whitespace-nowrap px-3 py-2">
                <span className="rounded-full bg-neutral-100 px-2 py-0.5 text-xs font-medium text-neutral-800 dark:bg-neutral-800 dark:text-neutral-200">
                  {d.status}
                </span>
              </td>
              <td className="whitespace-nowrap px-3 py-2 tabular-nums text-neutral-800 dark:text-neutral-200">
                {formatMoney(d.total, d.currency)}
              </td>
              <td className="whitespace-nowrap px-3 py-2 text-neutral-600 dark:text-neutral-400">
                {formatDate(d.issue_date)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
