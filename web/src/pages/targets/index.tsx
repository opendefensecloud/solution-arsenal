import { useQuery } from "@tanstack/react-query";
import { targetQueries, releaseBindingQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";
import { Server } from "lucide-react";

const namespace = "default"; // TODO: namespace selector

export function TargetsPage() {
  useSSE(namespace);

  const { data, isLoading } = useQuery(targetQueries.list(namespace));
  const { data: bindings } = useQuery(
    releaseBindingQueries.list(namespace),
  );

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Server className="h-4 w-4 animate-pulse" />
        Loading targets...
      </div>
    );
  }

  const targets = data?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">Targets</h1>
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
                No targets found in namespace &ldquo;{namespace}&rdquo;
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {targets.map((target) => {
            const targetBindings =
              bindings?.items.filter(
                (b) => b.spec.targetRef.name === target.metadata.name,
              ) ?? [];

            return (
              <Card key={target.metadata.name} className="p-4">
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
