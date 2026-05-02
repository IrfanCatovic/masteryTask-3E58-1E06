import { useEffect, useState } from 'react'
import { apiUrl } from '../api/client'


type HealthState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'ok'; body: unknown }
  | { kind: 'error'; message: string }

const shell =
  'rounded-lg border px-4 py-3 text-sm leading-relaxed'

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
      <div className={`${shell} border-neutral-200 bg-neutral-50 text-neutral-600 dark:border-neutral-700 dark:bg-neutral-900/40 dark:text-neutral-400`} role="status">
        Provera API-ja…
      </div>
    )
  }

  if (state.kind === 'error') {
    return (
      <div
        className={`${shell} border-red-200 bg-red-50 text-red-900 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-200`}
        role="alert"
      >
        <strong className="font-semibold">API nije dostupan.</strong> {state.message}
      </div>
    )
  }

  return (
    <div
      className={`${shell} border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-100`}
      role="status"
    >
      <strong className="font-semibold">API živ.</strong>{' '}
      <code className="ml-1 break-all font-mono text-xs opacity-90">
        {JSON.stringify(state.body)}
      </code>
    </div>
  )
}
