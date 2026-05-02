//ovde su tipovi za dokumente

export type LineItem = {
  id: number
  document_id: number
  description: string
  quantity: number
  unit_price: number
  line_total: number
  created_at: string
  updated_at: string
}

export type ValidationIssue = {
  id: number
  document_id: number
  code: string
  message: string
  severity: string
  field_name?: string
  resolved: boolean
  created_at: string
  updated_at: string
}

export type Document = {
  id: number
  document_type: string
  supplier_name: string
  document_number: string
  status: string
  issue_date?: string | null
  due_date?: string | null
  currency?: string
  subtotal?: number
  tax_rate?: number
  discount_rate?: number
  total?: number
  line_items?: LineItem[]
  issues?: ValidationIssue[]
  created_at: string
  updated_at: string
}

export type DocumentsListResponse = {
  status: string
  count: number
  documents: Document[]
}

export const DOCUMENT_STATUSES = [
  'uploaded',
  'needs_review',
  'validated',
  'rejected',
] as const

export type DocumentStatus = (typeof DOCUMENT_STATUSES)[number]
