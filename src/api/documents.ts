import { apiUrl } from './client'
import type { Document, DocumentsListResponse, DocumentStatus } from '../types/document'

// Extracts a human-readable error from a failed JSON response so the UI shows
// "csv missing required column: document_type" instead of the full JSON body.
async function readApiError(res: Response): Promise<string> {
  const text = await res.text()
  if (!text) return `HTTP ${res.status} ${res.statusText}`
  try {
    const body = JSON.parse(text) as { error?: string; message?: string }
    return body.error || body.message || text
  } catch {
    return text
  }
}

export async function fetchDocuments(status?: string): Promise<Document[]> {
  const search = new URLSearchParams()
  if (status !== undefined && status !== '') {
    search.set('status', status)
  }
  const query = search.toString()
  const path = query ? `/documents?${query}` : '/documents'
  const res = await fetch(apiUrl(path))
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  const data = (await res.json()) as DocumentsListResponse
  return data.documents ?? []
}

export async function fetchDocument(id: number): Promise<Document> {
  const res = await fetch(apiUrl(`/documents/${id}`))
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  const data = (await res.json()) as { status: string; document: Document }
  return data.document
}

export async function patchDocumentStatus(id: number, status: DocumentStatus): Promise<Document> {
  const res = await fetch(apiUrl(`/documents/${id}/status`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  })
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  const data = (await res.json()) as { document: Document }
  return data.document
}

// Payload for manual corrections; only fields the user actually changed.
export type UpdateDocumentPayload = Partial<{
  document_type: string
  supplier_name: string
  document_number: string
  issue_date: string
  due_date: string
  currency: string
  subtotal: number
  tax_rate: number
  discount_rate: number
  total: number
}>

export async function deleteDocument(id: number): Promise<void> {
  const res = await fetch(apiUrl(`/documents/${id}`), { method: 'DELETE' })
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
}

export async function updateDocument(id: number, payload: UpdateDocumentPayload): Promise<Document> {
  const res = await fetch(apiUrl(`/documents/${id}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  const data = (await res.json()) as { document: Document }
  return data.document
}

export type UploadDocumentResult = {
  document: Document
  issues_count: number
}

/** Thrown when the API returns 409 — document number already in the database. */
export class DuplicateDocumentError extends Error {
  readonly code = 'DUPLICATE_DOCUMENT_NUMBER' as const
  constructor(message: string) {
    super(message)
    this.name = 'DuplicateDocumentError'
  }
}

export async function uploadDocument(file: File): Promise<UploadDocumentResult> {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(apiUrl('/documents/upload'), {
    method: 'POST',
    body: form,
  })
  if (res.status === 409) {
    await res.text().catch(() => undefined)
    throw new DuplicateDocumentError('A document with this number already exists.')
  }
  if (!res.ok) {
    throw new Error(await readApiError(res))
  }
  return (await res.json()) as UploadDocumentResult
}
