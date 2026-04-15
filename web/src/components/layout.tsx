import { Link, useRouterState } from "@tanstack/react-router";
import {
    LayoutDashboard,
    Server,
    Package,
    Boxes,
    Users,
    LogOut,
    ShieldCheck,
    Eye,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import {
    useAuth,
    ROLE_LABELS,
    IMPERSONATABLE_USERS,
    type Permissions,
} from "@/hooks/useAuth";

const navItems: {
    to: string;
    label: string;
    icon: React.ElementType;
    visible?: (p: Permissions) => boolean;
}[] = [
    { to: "/", label: "Dashboard", icon: LayoutDashboard },
    {
        to: "/targets",
        label: "Targets",
        icon: Server,
        visible: (p) => p.canViewDeployments,
    },
    {
        to: "/releases",
        label: "Releases",
        icon: Package,
        visible: (p) => p.canViewDeployments,
    },
    {
        to: "/components",
        label: "Components",
        icon: Boxes,
        visible: (p) => p.canViewCatalog,
    },
    {
        to: "/profiles",
        label: "Profiles",
        icon: Users,
        visible: (p) => p.canViewDeployments,
    },
] as const;

export function Layout({ children }: { children: React.ReactNode }) {
    const {
        user,
        permissions,
        isImpersonating,
        impersonatedUsername,
        impersonate,
        clearImpersonation,
    } = useAuth();
    const router = useRouterState();
    const currentPath = router.location.pathname;

    const visibleNav = navItems.filter(
        (item) => !item.visible || item.visible(permissions),
    );

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
                    {visibleNav.map(({ to, label, icon: Icon }) => {
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

                {/* Footer: theme toggle + user + preview-as */}
                <div className="border-t border-sidebar-border p-3 space-y-2">
                    <div className="flex items-center justify-between px-1">
                        <span className="text-[11px] font-medium text-muted-foreground">
                            Theme
                        </span>
                        <ThemeToggle />
                    </div>

                    {/* Preview as — only shown to platform-admin */}
                    {user?.authenticated && permissions.isAdmin && (
                        <div className="space-y-1">
                            <p className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1">
                                <Eye className="h-3 w-3" />
                                Preview as
                            </p>
                            <select
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
                                {IMPERSONATABLE_USERS.map((u) => (
                                    <option key={u.username} value={u.username}>
                                        {u.label}
                                    </option>
                                ))}
                            </select>
                            {isImpersonating && (
                                <p className="px-1 text-[10px] text-amber-600 dark:text-amber-400">
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
                                <p className="flex items-center gap-1 truncate text-xs text-muted-foreground">
                                    <ShieldCheck className="h-3 w-3 shrink-0" />
                                    {ROLE_LABELS[permissions.role]}
                                </p>
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

