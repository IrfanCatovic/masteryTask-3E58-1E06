import { useEffect, useState } from 'react'
import { apiUrl } from '../api/client'

type HealthState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'ok'; body: unknown }
  | { kind: 'error'; message: string }

export function ApiHealthBadge() {
  const [state, setState] = useState<HealthState>({ kind: 'idle' })

  useEffect(() => {
    const ctrl = new AbortController()
    setState({ kind: 'loading' })

    fetch(apiUrl('/health'), { signal: ctrl.signal })
      .then(async (res) => {
        if (!res.ok) {
          setState({
            kind: 'error',
            message: `HTTP ${res.status} ${res.statusText}`,
          })
          return
        }
        const body = await res.json().catch(() => ({}))
        setState({ kind: 'ok', body })
      })
      .catch((e: unknown) => {
        const message =
          e instanceof Error ? e.message : 'Failed to reach backend (is it running on :8080?)'
        setState({ kind: 'error', message })
      })

    return () => ctrl.abort()
  }, [])

  if (state.kind === 'loading' || state.kind === 'idle') {
    return (
      <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/80 px-3 py-1.5 text-xs font-medium text-slate-600 shadow-sm">
        <span className="h-2 w-2 animate-pulse rounded-full bg-amber-500" />
        Checking API
      </div>
    )
  }

  if (state.kind === 'error') {
    return (
      <div
        className="inline-flex max-w-full items-center gap-2 rounded-full border border-red-200 bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 shadow-sm"
        role="alert"
        title={state.message}
      >
        <span className="h-2 w-2 rounded-full bg-red-500" />
        API offline
      </div>
    )
  }

  return (
    <div
      className="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-medium text-emerald-700 shadow-sm"
      role="status"
      title={JSON.stringify(state.body)}
    >
      <span className="h-2 w-2 rounded-full bg-emerald-500" />
      API connected
    </div>
  )
}
