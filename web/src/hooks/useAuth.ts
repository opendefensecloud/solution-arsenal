import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { authQueries } from "@/api/queries";

export interface ImpersonationRequest {
    username: string;
    groups?: string[];
}

/**
 * useAuth wraps the /auth/me query and the admin-only impersonation mutations.
 *
 * Authorization (what data the user can see, edit, delete) is intentionally
 * NOT modelled here — the BFF forwards the OIDC token (or impersonation
 * headers) to the K8s API server and authorization is decided by cluster RBAC.
 * The frontend only needs to know:
 *   1. Who is the user, and as whom are they currently acting (for display).
 *   2. May they impersonate? (cluster-level SelfSubjectAccessReview)
 *
 * The set of users an admin may impersonate is also K8s' job — the BFF
 * accepts any username/groups the admin types and lets the cluster reject
 * unauthorised combinations at request time.
 */
export function useAuth() {
    const queryClient = useQueryClient();
    const { data: user, isLoading } = useQuery(authQueries.me());

    const isAuthenticated = user?.authenticated ?? false;
    const isAdmin = user?.canImpersonate ?? false;
    const canListAllNamespaces = user?.canListAllNamespaces ?? false;
    const isImpersonating = !!user?.impersonating;

    // After flipping the BFF's session impersonation we have to drop every
    // cached resource list — they were fetched as the previous identity and
    // may have been filtered by RBAC. resetQueries() removes the data and
    // puts active queries back into the "pending" state, so the UI shows
    // loading skeletons instead of stale rows until the BFF responds for
    // the new identity. Returning the promise keeps the mutation
    // "pending" until refetches settle.
    const resetAll = () => queryClient.resetQueries();

    const impersonateMutation = useMutation({
        mutationFn: (req: ImpersonationRequest) =>
            api.put<void>("/auth/impersonate", req),
        onSuccess: resetAll,
    });

    const clearImpersonationMutation = useMutation({
        mutationFn: () => api.delete<void>("/auth/impersonate"),
        onSuccess: resetAll,
    });

    return {
        user,
        isLoading,
        isAuthenticated,
        isAdmin,
        canListAllNamespaces,
        isImpersonating,
        impersonatedUsername: user?.impersonating?.username,
        impersonatedGroups: user?.impersonating?.groups,
        impersonate: impersonateMutation.mutate,
        clearImpersonation: clearImpersonationMutation.mutate,
        isImpersonatePending: impersonateMutation.isPending || clearImpersonationMutation.isPending,
    };
}
