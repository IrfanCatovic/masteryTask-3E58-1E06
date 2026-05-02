import { ApiHealthBadge } from './components/ApiHealthBadge'

function App() {
  return (
    <main className="mx-auto max-w-xl px-4 py-10">
      <h1 className="mb-2 text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
        Smart Document Processing
      </h1>
      <p className="mb-6 text-sm leading-relaxed text-neutral-600 dark:text-neutral-400">
        Prvi korak: spajanje fronta i Go API-ja. Pokreni backend na portu 8080, zatim{' '}
        <code className="rounded bg-neutral-100 px-1.5 py-0.5 font-mono text-[0.85em] dark:bg-neutral-800">
          npm run dev
        </code>
        .
      </p>
      <ApiHealthBadge />
    </main>
  )
}

export default App
