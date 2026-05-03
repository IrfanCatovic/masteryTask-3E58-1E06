import { useState, type FormEvent } from 'react'
import { DuplicateDocumentError, uploadDocument } from '../api/documents'
import type { Document } from '../types/document'

type Props = {
  onSuccess: (document: Document) => void
  /** Called when upload is rejected because document_number already exists (no row inserted). */
  onDuplicate?: () => void
}

export function UploadForm({ onSuccess, onDuplicate }: Props) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const input = e.currentTarget.elements.namedItem('file') as HTMLInputElement
    const file = input.files?.[0]
    if (!file) {
      setError('Choose a CSV, TXT, PDF, or image file.')
      return
    }
    setBusy(true)
    setError(null)
    setInfo(null)
    try {
      const result = await uploadDocument(file)
      if (result.issues_count > 0) {
        setInfo(
          `Uploaded with ${result.issues_count} issue${result.issues_count === 1 ? '' : 's'} — open the document to review and correct.`,
        )
      } else {
        setInfo('Uploaded successfully — no issues detected.')
      }
      onSuccess(result.document)
      input.value = ''
    } catch (err: unknown) {
      if (err instanceof DuplicateDocumentError) {
        onDuplicate?.()
        return
      }
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      onSubmit={(e) => void onSubmit(e)}
      className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"
    >
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="min-w-0 flex-1">
          <label htmlFor="upload-file" className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
            Upload document
          </label>
          <p className="mt-1 text-sm text-slate-500">
            CSV, TXT, PDF, or images (PNG/JPEG/WebP) — field name{' '}
            <code className="font-mono text-slate-700">file</code>. Images are sent to OCR on the
            server (configure <code className="font-mono text-slate-700">OCR_SPACE_API_KEY</code> on
            the API). PDFs use embedded text when available.
          </p>
          <input
            id="upload-file"
            name="file"
            type="file"
            accept=".csv,.txt,.pdf,.png,.jpg,.jpeg,.webp,text/csv,text/plain,application/pdf,image/png,image/jpeg,image/webp"
            disabled={busy}
            className="mt-3 block w-full text-sm text-slate-700 file:mr-4 file:rounded-lg file:border-0 file:bg-slate-900 file:px-4 file:py-2 file:text-sm file:font-semibold file:text-white hover:file:bg-slate-800"
          />
        </div>
        <button
          type="submit"
          disabled={busy}
          className="shrink-0 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy ? 'Uploading…' : 'Upload'}
        </button>
      </div>
      {error && (
        <p className="mt-3 text-sm font-medium text-red-700" role="alert">
          {error}
        </p>
      )}
      {info && !error && (
        <p className="mt-3 text-sm font-medium text-emerald-700">{info}</p>
      )}
    </form>
  )
}
