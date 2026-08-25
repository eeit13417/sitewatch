import { Activity } from "lucide-react"
import { NavLink } from "react-router"
import { useAlerts } from "../hooks/useAlerts"
import { cn } from "@/lib/utils"

const links = [
  { to: "/", label: "Sites" },
  { to: "/alerts", label: "Alerts" },
]

export function NavBar() {
  // Unfiltered open-alert count for the nav badge — same query the Alerts
  // page's default filter uses, so the two numbers can never disagree.
  const openAlerts = useAlerts({ status: "open" })
  const openCount = openAlerts.data?.length ?? 0

  return (
    <header className="sticky top-0 z-20 border-b border-border bg-background/85 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-7xl items-center gap-6 px-4 sm:px-6">
        <NavLink to="/" className="flex items-center gap-2">
          <span className="flex size-7 items-center justify-center rounded bg-primary text-primary-foreground">
            <Activity className="size-4" aria-hidden />
          </span>
          <span className="text-sm font-semibold tracking-tight text-foreground">SiteWatch</span>
        </NavLink>

        <nav className="flex items-center gap-1" aria-label="Primary">
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              end
              className={({ isActive }) =>
                cn(
                  "relative rounded px-3 py-1.5 text-sm font-medium transition-colors",
                  isActive ? "bg-surface-2 text-foreground" : "text-muted hover:text-foreground",
                )
              }
            >
              {link.label}
              {link.to === "/alerts" && openCount > 0 && (
                <span
                  data-testid="nav-open-alert-count"
                  className="ml-1.5 inline-flex min-w-4 items-center justify-center rounded-full bg-critical px-1 text-[10px] font-semibold text-critical-foreground tabular-nums"
                >
                  {openCount}
                </span>
              )}
            </NavLink>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-2 text-xs text-faint">
          <span className="flex size-2 items-center justify-center">
            <span className="size-2 animate-pulse rounded-full bg-ok" />
          </span>
          Live
        </div>
      </div>
    </header>
  )
}
