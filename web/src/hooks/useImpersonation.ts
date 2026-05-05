import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authQueries, impersonationQueries, impersonationMutations } from "@/api/queries";

export interface UseImpersonationResult {
  /** Available personas fetched from the BFF (empty until loaded). */
  targets: { username: string; groups: string[] }[];
  /** Username currently being impersonated, or null when acting as self. */
  impersonatedUsername: string | null;
  isImpersonating: boolean;
  /** Activate impersonation for the given username (must be a known target). */
  impersonate: (username: string) => void;
  /** Revert to acting as the real admin user. */
  clearImpersonation: () => void;
}

export function useImpersonation(): UseImpersonationResult {
  const queryClient = useQueryClient();
  const { data: user } = useQuery(authQueries.me());
  const { data: targets = [] } = useQuery(impersonationQueries.targets());

  const impersonatedUsername = user?.impersonating?.username ?? null;

  // After either mutation resolves, invalidate /auth/me and permissions so the
  // UI immediately reflects the new effective identity.
  const onSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
    queryClient.invalidateQueries({ queryKey: ["permissions"] });
  };

  const impersonateMutation = useMutation({
    mutationFn: (username: string) => impersonationMutations.impersonate(username),
    onSuccess,
  });

  const clearMutation = useMutation({
    mutationFn: () => impersonationMutations.clear(),
    onSuccess,
  });

  return {
    targets,
    impersonatedUsername,
    isImpersonating: impersonatedUsername !== null,
    impersonate: (username) => impersonateMutation.mutate(username),
    clearImpersonation: () => clearMutation.mutate(),
  };
}
