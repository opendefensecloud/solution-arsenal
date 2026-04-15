import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { authQueries } from "@/api/queries";

export type Role =
    | "platform-admin"
    | "solution-maintainer"
    | "deployment-coordinator";

export interface Permissions {
    role: Role;
    canViewCatalog: boolean;
    canEditCatalog: boolean;
    canViewDeployments: boolean;
    canEditDeployments: boolean;
    // not affected by "preview as"
    isAdmin: boolean;
    isImpersonating: boolean;
}

export interface ImpersonatableUser {
    username: string;
    label: string;
    role: Role;
}

export const IMPERSONATABLE_USERS: ImpersonatableUser[] = [
    {
        username: "maintainer@solar.local",
        label: "Solution Maintainer",
        role: "solution-maintainer",
    },
    {
        username: "coordinator@solar.local",
        label: "Deployment Coordinator",
        role: "deployment-coordinator",
    },
];

export const ROLE_LABELS: Record<Role, string> = {
    "platform-admin": "Platform Admin",
    "solution-maintainer": "Solution Maintainer",
    "deployment-coordinator": "Deployment Coordinator",
};

const GROUP_TO_ROLE: Record<string, Role> = {
    admin: "platform-admin",
    maintainer: "solution-maintainer",
    coordinator: "deployment-coordinator",
};

const USERNAME_TO_ROLE: Record<string, Role> = {
    "admin@solar.local": "platform-admin",
    "maintainer@solar.local": "solution-maintainer",
    "coordinator@solar.local": "deployment-coordinator",
};

function deriveRole(username: string, groups: string[]): Role {
    for (const g of groups) {
        const mapped = GROUP_TO_ROLE[g];
        if (mapped) return mapped;
    }
    return USERNAME_TO_ROLE[username] ?? "";
}

function buildPermissions(role: Role, isImpersonating: boolean): Permissions {
    switch (role) {
        case "platform-admin":
            return {
                role,
                canViewCatalog: true,
                canEditCatalog: true,
                canViewDeployments: true,
                canEditDeployments: true,
                isAdmin: true,
                isImpersonating,
            };
        case "solution-maintainer":
            return {
                role,
                canViewCatalog: false,
                canEditCatalog: false,
                canViewDeployments: true,
                canEditDeployments: true,
                isAdmin: false,
                isImpersonating,
            };
        case "deployment-coordinator":
            return {
                role,
                canViewCatalog: false,
                canEditCatalog: false,
                canViewDeployments: true,
                canEditDeployments: true,
                isAdmin: false,
                isImpersonating,
            };
    }
}

export function useAuth() {
    const queryClient = useQueryClient();
    const { data: user, isLoading } = useQuery(authQueries.me());

    const impersonateMutation = useMutation({
        mutationFn: (username: string) =>
            api.put<void>("/auth/impersonate", { username }),
        onSuccess: () =>
            queryClient.invalidateQueries({ queryKey: ["auth", "me"] }),
    });

    const clearImpersonationMutation = useMutation({
        mutationFn: () => api.delete<void>("/auth/impersonate"),
        onSuccess: () =>
            queryClient.invalidateQueries({ queryKey: ["auth", "me"] }),
    });

    const isAuthenticated = user?.authenticated ?? false;
    const isImpersonating = !!user?.impersonating;

    // Derive role from the active identity (impersonated or real)
    const activeUsername = user?.impersonating?.username ?? user?.username ?? "";
    const activeGroups = user?.impersonating?.groups ?? user?.groups ?? [];
    const activeRole = deriveRole(activeUsername, activeGroups);

    // isAdmin always reflects the real identity — an admin impersonating
    // another role can still see the "Preview as" controls
    const realRole = deriveRole(user?.username ?? "", user?.groups ?? []);
    const permissions: Permissions = {
        ...buildPermissions(activeRole, isImpersonating),
        isAdmin: realRole === "platform-admin",
    };

    return {
        user,
        isLoading,
        isAuthenticated,
        permissions,
        impersonate: impersonateMutation.mutate,
        clearImpersonation: clearImpersonationMutation.mutate,
        isImpersonating,
        impersonatedUsername: user?.impersonating?.username,
    };
}
