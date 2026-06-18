import { useQuery } from "@tanstack/react-query";
import { targetQueries, releaseBindingQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";
import { useNamespace } from "@/hooks/useNamespace";
import { ForbiddenAllNs } from "@/components/forbidden-all-ns";
import { isForbiddenError } from "@/api/client";
import { Server } from "lucide-react";

export function TargetsPage() {
  const { namespace } = useNamespace();
  useSSE(namespace);

  const { data, isLoading, error } = useQuery(targetQueries.list(namespace));
  const { data: bindings } = useQuery(
    releaseBindingQueries.list(namespace),
  );

  if (namespace === null && isForbiddenError(error)) {
    return <ForbiddenAllNs resource="targets" />;
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Server className="h-4 w-4 animate-pulse" />
        Loading targets...
      </div>
    );
  }

  if (error) {
    return (
      <Card className="py-8">
        <CardContent>
          <p className="text-sm text-destructive">
            Failed to load targets. Please retry.
          </p>
        </CardContent>
      </Card>
    );
  }

  const targets = data?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Targets</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace{" "}
            <span className="font-mono">{namespace ?? "all"}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {targets.length} target{targets.length !== 1 ? "s" : ""}
        </span>
      </div>

      {targets.length === 0 ? (
        <Card className="py-12">
          <CardContent>
            <div className="text-center">
              <Server className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
              <p className="text-muted-foreground">
                No targets found in namespace &ldquo;{namespace ?? "all"}&rdquo;
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {targets.map((target) => {
            const targetBindings =
              bindings?.items.filter(
                (b) =>
                  b.spec.targetRef.name === target.metadata.name &&
                  (namespace !== null ||
                    b.metadata.namespace === target.metadata.namespace),
              ) ?? [];

            return (
              <Card
                key={`${target.metadata.namespace ?? "unknown"}/${target.metadata.name}`}
                className="p-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-base">
                      {target.metadata.name}
                    </CardTitle>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Registry:{" "}
                      <span className="font-mono">
                        {target.spec.renderRegistryRef.name}
                      </span>
                      {" | "}
                      Age: {formatAge(target.metadata.creationTimestamp)}
                      {targetBindings.length > 0 && (
                        <>
                          {" | "}
                          {targetBindings.length} release
                          {targetBindings.length !== 1 ? "s" : ""} bound
                        </>
                      )}
                    </p>
                  </div>
                  <StatusBadge conditions={target.status?.conditions} />
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
