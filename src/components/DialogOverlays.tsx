import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

const shellClass =
  'max-w-md rounded-3xl border border-slate-200 bg-white p-6 shadow-2xl shadow-slate-900/20'
/** Full-viewport overlay mounted on document.body so parent overflow/transform cannot clip it. */
const backdropClass =
  'fixed inset-0 z-[100] flex items-center justify-center bg-slate-950/60 p-4 backdrop-blur-sm'

type ConfirmDialogProps = {
  open: boolean
  title: string
  description: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  /** Red destructive styling for the confirm button (e.g. delete). */
  danger?: boolean
  loading?: boolean
  onConfirm: () => void
  onClose: () => void
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger,
  loading,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  if (!open) return null

  const node = (
    <div className={backdropClass} role="presentation" onClick={() => !loading && onClose()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        className={shellClass}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 id="confirm-dialog-title" className="text-lg font-semibold text-slate-950">
          {title}
        </h3>
        <div className="mt-3 text-sm leading-relaxed text-slate-600">{description}</div>
        <div className="mt-6 flex flex-wrap justify-end gap-3">
          <button
            type="button"
            disabled={loading}
            onClick={onClose}
            className="rounded-xl border border-slate-200 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            disabled={loading}
            onClick={() => onConfirm()}
            className={`rounded-xl px-5 py-2.5 text-sm font-semibold text-white shadow-sm disabled:cursor-not-allowed disabled:opacity-50 ${
              danger
                ? 'bg-red-600 hover:bg-red-500'
                : 'bg-slate-950 hover:bg-slate-800'
            }`}
          >
            {loading ? 'Please wait…' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )

  return createPortal(node, document.body)
}

type AlertDialogProps = {
  open: boolean
  title: string
  message: string
  buttonLabel?: string
  onClose: () => void
}

export function AlertDialog({
  open,
  title,
  message,
  buttonLabel = 'OK',
  onClose,
}: AlertDialogProps) {
  if (!open) return null

  const node = (
    <div className={backdropClass} role="presentation" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="alert-dialog-title"
        className={shellClass}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 id="alert-dialog-title" className="text-lg font-semibold text-slate-950">
          {title}
        </h3>
        <p className="mt-3 text-sm leading-relaxed text-slate-600">{message}</p>
        <div className="mt-6 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl bg-slate-950 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-slate-800"
          >
            {buttonLabel}
          </button>
        </div>
      </div>
    </div>
  )

  return createPortal(node, document.body)
}
