import { SearchX } from "lucide-react"
import { Link } from "react-router"

export function NotFound({ message = "Page not found." }: { message?: string }) {
  return (
    <div className="mx-auto flex max-w-7xl flex-col items-center px-4 py-24 text-center sm:px-6">
      <span className="flex size-12 items-center justify-center rounded-full border border-border bg-surface text-faint">
        <SearchX className="size-6" aria-hidden />
      </span>
      <p className="mt-4 text-sm text-muted">{message}</p>
      <Link
        to="/"
        className="mt-4 rounded border border-border-strong bg-surface-2 px-3 py-1.5 text-sm font-medium text-foreground hover:border-primary/60 hover:text-primary"
      >
        Back to Sites
      </Link>
    </div>
  )
}
