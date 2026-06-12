import { Card, CardContent } from "@/components/ui/card";
import { Lock } from "lucide-react";

/**
 * Shown when a list query 403s in "All namespaces" mode. K8s requires
 * cluster-scoped `list <resource>` permission for the cluster-wide
 * endpoint; users bound only to specific-namespace Roles can't do that.
 * Suggestion: pick a specific namespace from the selector.
 */
export function ForbiddenAllNs({ resource }: { resource: string }) {
    return (
        <Card className="py-10">
            <CardContent>
                <div className="mx-auto max-w-md text-center">
                    <Lock className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
                    <p className="font-medium text-foreground">
                        No cluster-wide access to {resource}
                    </p>
                    <p className="mt-1 text-sm text-muted-foreground">
                        Listing {resource} across all namespaces requires
                        cluster-scope RBAC, which your current identity doesn't
                        have. Pick a specific namespace from the sidebar to see
                        what you can access.
                    </p>
                </div>
            </CardContent>
        </Card>
    );
}
