import { useQuery } from "@tanstack/react-query";
import { authQueries, permissionQueries } from "@/api/queries";
import type { PermissionsResponse, PolicyRule } from "@/api/types";

/**
 * Returns true if any rule in the list grants `verb` on `resource` in `apiGroup`.
 * Wildcards ("*") in verbs, resources, or apiGroups are honoured.
 *
 * When `apiGroup` is omitted the check is group-agnostic (matches any rule that
 * grants the verb+resource regardless of group).
 */
export function canDo(
  rules: PolicyRule[],
  verb: string,
  resource: string,
  apiGroup?: string,
): boolean {
  for (const rule of rules) {
    const verbMatch =
      rule.verbs.includes("*") || rule.verbs.includes(verb);
    const resourceMatch =
      rule.resources.includes("*") || rule.resources.includes(resource);
    const groupMatch =
      apiGroup === undefined ||
      rule.apiGroups.includes("*") ||
      rule.apiGroups.includes(apiGroup);

    if (verbMatch && resourceMatch && groupMatch) {
      return true;
    }
  }
  return false;
}

export type UsePermissionsResult =
  | { ready: false; isAdmin: false; can: () => false }
  | {
      ready: true;
      incomplete: boolean;
      isAdmin: boolean;
      /** Check whether the current user may perform `verb` on `resource`. */
      can: (verb: string, resource: string, apiGroup?: string) => boolean;
    };

/**
 * Fetches and exposes the current user's RBAC permissions for `namespace`.
 */
export function usePermissions(namespace: string): UsePermissionsResult {
  const { data } = useQuery(permissionQueries.rules(namespace));
  const { data: user } = useQuery(authQueries.me());
  // isAdmin is determined by the BFF 
  const isAdmin = user?.isAdmin ?? false;

  if (!data) {
    return { ready: false, isAdmin: false, can: () => false };
  }

  return {
    ready: true,
    isAdmin,
    incomplete: data.incomplete,
    can: (verb, resource, apiGroup) =>
      canDo(data.rules, verb, resource, apiGroup),
  };
}

/**
 * Checks permissions from an already-fetched PermissionsResponse.
 * Used in TanStack Router `beforeLoad` where we have the raw data.
 */
export function permissionsFromResponse(
  data: PermissionsResponse,
): (verb: string, resource: string, apiGroup?: string) => boolean {
  return (verb, resource, apiGroup) =>
    canDo(data.rules, verb, resource, apiGroup);
}
