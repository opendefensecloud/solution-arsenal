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
import { ThemeToggle } from "@/components/ui/theme-toggle";

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
            <aside className="flex w-60 flex-col border-r border-sidebar-border bg-sidebar">
                {/* Logo */}
                <div className="flex h-14 items-center gap-2.5 border-b border-sidebar-border px-4">
                    <img src="/solar.svg" alt="SolAr" className="h-14 w-14" />
                    <div>
                        <span className="text-lg font-bold tracking-tight text-primary">
                            SolAr
                        </span>
                        <p className="text-[12px] leading-none text-muted-foreground">
                            Solution Arsenal
                        </p>
                    </div>
                </div>

                {/* Navigation */}
                <nav className="flex-1 space-y-0.5 p-3">
                    <p className="mb-2 px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        Navigation
                    </p>
                    {navItems.map(({ to, label, icon: Icon }) => {
                        const isActive =
                            currentPath === to ||
                            (to !== "/" && currentPath.startsWith(to));

                        return (
                            <Link
                                key={to}
                                to={to}
                                className={cn(
                                    "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                                    isActive
                                        ? "bg-primary/10 text-primary"
                                        : "text-sidebar-foreground hover:bg-accent hover:text-accent-foreground",
                                )}
                            >
                                <Icon
                                    className={cn(
                                        "h-4 w-4",
                                        isActive ? "text-primary" : "text-muted-foreground",
                                    )}
                                />
                                {label}
                            </Link>
                        );
                    })}
                </nav>

                {/* Footer: theme toggle + user */}
                <div className="border-t border-sidebar-border p-3">
                    <div className="mb-3 flex items-center justify-between px-1">
                        <span className="text-[11px] font-medium text-muted-foreground">
                            Theme
                        </span>
                        <ThemeToggle />
                    </div>
                    {user?.authenticated && (
                        <div className="flex items-center justify-between rounded-md bg-accent/50 px-3 py-2">
                            <div className="min-w-0">
                                <p className="truncate text-sm font-medium text-foreground">
                                    {user.username}
                                </p>
                                {user.groups?.[0] && (
                                    <p className="truncate text-xs text-muted-foreground">
                                        {user.groups[0]}
                                    </p>
                                )}
                            </div>
                            <a
                                href="/api/auth/logout"
                                className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                                title="Logout"
                            >
                                <LogOut className="h-4 w-4" />
                            </a>
                        </div>
                    )}
                </div>
            </aside>

            {/* Main content */}
            <main className="flex-1 overflow-auto">
                <div className="mx-auto max-w-7xl px-8 py-8">{children}</div>
            </main>
        </div>
    );
}
