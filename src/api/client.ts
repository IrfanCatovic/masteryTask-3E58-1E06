
export function getApiBase(): string {
  const raw = import.meta.env.VITE_API_URL as string | undefined
  if (raw !== undefined && raw.trim() !== '') {
    return raw.replace(/\/$/, '')
  }
  return '/api'
}


export function apiUrl(path: string): string {
  const base = getApiBase()
  const p = path.startsWith('/') ? path : `/${path}`
  return `${base}${p}`
}
