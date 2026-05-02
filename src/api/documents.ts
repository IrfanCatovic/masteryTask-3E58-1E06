import { apiUrl } from './client'
import type { Document, DocumentsListResponse, DocumentStatus } from '../types/document'

export async function fetchDocuments(status?: string): Promise<Document[]> {
  const search = new URLSearchParams()
  if (status !== undefined && status !== '') {
    search.set('status', status)
  }
  const query = search.toString()
  const path = query ? `/documents?${query}` : '/documents'
  const res = await fetch(apiUrl(path))
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`)
  }
  const data = (await res.json()) as DocumentsListResponse
  return data.documents ?? []
}

export async function fetchDocument(id: number): Promise<Document> {
  const res = await fetch(apiUrl(`/documents/${id}`))
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`)
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
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`)
  }
  const data = (await res.json()) as { document: Document }
  return data.document
}

export type UploadDocumentResult = {
  document: Document
  issues_count: number
}

export async function uploadDocument(file: File): Promise<UploadDocumentResult> {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(apiUrl('/documents/upload'), {
    method: 'POST',
    body: form,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`)
  }
  return (await res.json()) as UploadDocumentResult
}
