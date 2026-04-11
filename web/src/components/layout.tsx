import { Link, useRouterState } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { authQueries } from "@/api/queries";
import {
  LayoutDashboard,
  Server,
  Package,
  Boxes,
  Users,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/targets", label: "Targets", icon: Server },
  { to: "/releases", label: "Releases", icon: Package },
  { to: "/components", label: "Components", icon: Boxes },
  { to: "/profiles", label: "Profiles", icon: Users },
] as const;

export function Layout({ children }: { children: React.ReactNode }) {
  const { data: user } = useQuery(authQueries.me());
  const router = useRouterState();
  const currentPath = router.location.pathname;

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="flex w-56 flex-col border-r bg-card">
        <div className="flex h-14 items-center border-b px-4">
          <span className="text-lg font-bold tracking-tight text-primary">
            SolAr
          </span>
        </div>

        <nav className="flex-1 space-y-1 p-2">
          {navItems.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                currentPath === to || (to !== "/" && currentPath.startsWith(to))
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </Link>
          ))}
        </nav>

        {/* User info */}
        {user?.authenticated && (
          <div className="border-t p-3">
            <div className="flex items-center justify-between">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-foreground">
                  {user.username}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {user.groups?.[0]}
                </p>
              </div>
              <a
                href="/api/auth/logout"
                className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
                title="Logout"
              >
                <LogOut className="h-4 w-4" />
              </a>
            </div>
          </div>
        )}
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-7xl px-6 py-6">{children}</div>
      </main>
    </div>
  );
}
