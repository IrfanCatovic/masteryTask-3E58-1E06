//ovde su funkcije za dokumente
import { apiUrl } from './client'
import type { Document, DocumentsListResponse } from '../types/document'

export async function fetchDocuments(status?: string): Promise<Document[]> {//fetchuje dokumente sa API-ja
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
