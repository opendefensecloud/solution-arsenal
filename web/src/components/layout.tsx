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
    Eye,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { usePermissions } from "@/hooks/usePermissions";
import { useImpersonation } from "@/hooks/useImpersonation";

const SOLAR_GROUP = "solar.opendefense.cloud";
const DEFAULT_NAMESPACE = "default";

// Each guarded nav item declares which verb+resource the user must have.
type NavItem = {
    to: string;
    label: string;
    icon: React.ElementType;
    guard: { verb: string; resource: string } | null;
};

const navItems: NavItem[] = [
    { to: "/", label: "Dashboard", icon: LayoutDashboard, guard: null },
    { to: "/targets", label: "Targets", icon: Server, guard: { verb: "list", resource: "targets" } },
    { to: "/releases", label: "Releases", icon: Package, guard: { verb: "list", resource: "releases" } },
    { to: "/components", label: "Components", icon: Boxes, guard: { verb: "list", resource: "components" } },
    { to: "/profiles", label: "Profiles", icon: Users, guard: { verb: "list", resource: "profiles" } },
];

export function Layout({ children }: { children: React.ReactNode }) {
    const { data: user } = useQuery(authQueries.me());
    const router = useRouterState();
    const currentPath = router.location.pathname;
    // potentially check permissions in specific namespaces in the future, but for now just check at the cluster level
    const permissions = usePermissions(DEFAULT_NAMESPACE);
    const { targets, impersonatedUsername, isImpersonating, impersonate, clearImpersonation } =
        useImpersonation();

    const visibleNavItems = navItems.filter(({ guard }) => {
        if (!guard) return true;
        return permissions.can(guard.verb, guard.resource, SOLAR_GROUP);
    });

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
                    {visibleNavItems.map(({ to, label, icon: Icon }) => {
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
                <div className="border-t border-sidebar-border p-3 space-y-3">
                    <div className="flex items-center justify-between px-1">
                        <span className="text-[11px] font-medium text-muted-foreground">
                            Theme
                        </span>
                        <ThemeToggle />
                    </div>

                    {/* Preview as — only shown to platform admins */}
                    {user?.authenticated && permissions.isAdmin && (
                        <div className="space-y-1">
                            <label
                                htmlFor="impersonation-select"
                                className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1"
                            >
                                <Eye className="h-3 w-3" aria-hidden="true" />
                                Preview as
                            </label>
                            <select
                                id="impersonation-select"
                                className={cn(
                                    "w-full rounded-md border px-2 py-1.5 text-xs bg-background text-foreground",
                                    isImpersonating
                                        ? "border-amber-500 text-amber-600 dark:text-amber-400"
                                        : "border-border",
                                )}
                                value={impersonatedUsername ?? ""}
                                onChange={(e) => {
                                    const val = e.target.value;
                                    if (val === "") {
                                        clearImpersonation();
                                    } else {
                                        impersonate(val);
                                    }
                                }}
                            >
                                <option value="">Myself (admin)</option>
                                {targets.map((t) => (
                                    <option key={t.username} value={t.username}>
                                        {t.username}
                                    </option>
                                ))}
                            </select>
                            {isImpersonating && (
                                <p role="status" className="px-1 text-[10px] text-amber-600 dark:text-amber-400">
                                    K8s requests run as {impersonatedUsername}
                                </p>
                            )}
                        </div>
                    )}

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
