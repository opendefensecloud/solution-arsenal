import { useQuery } from "@tanstack/react-query";
import { componentQueries, componentVersionQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";
import { useNamespace } from "@/hooks/useNamespace";
import { Badge } from "@/components/ui/badge";
import { ForbiddenAllNs } from "@/components/forbidden-all-ns";
import { isForbiddenError } from "@/api/client";
import { Boxes } from "lucide-react";

export function ComponentsPage() {
  const { namespace } = useNamespace();
  useSSE(namespace);

  const { data, isLoading, isError, error } = useQuery(
    componentQueries.list(namespace),
  );
  const {
    data: versionsData,
    isError: isVersionsError,
    error: versionsError,
  } = useQuery(componentVersionQueries.list(namespace));

  if (
    namespace === null &&
    (isForbiddenError(error) || isForbiddenError(versionsError))
  ) {
    return <ForbiddenAllNs resource="components" />;
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Boxes className="h-4 w-4 animate-pulse" />
        Loading components...
      </div>
    );
  }

  if (isError || isVersionsError) {
    return (
      <Card className="py-8">
        <CardContent>
          <p className="text-sm text-destructive">
            Failed to load components. Please retry.
          </p>
        </CardContent>
      </Card>
    );
  }

  const components = data?.items ?? [];
  const versions = versionsData?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Components</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace{" "}
            <span className="font-mono">{namespace ?? "all"}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {components.length} component{components.length !== 1 ? "s" : ""}
        </span>
      </div>

      {components.length === 0 ? (
        <Card className="py-12">
          <CardContent>
            <div className="text-center">
              <Boxes className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
              <p className="text-muted-foreground">
                No components discovered yet
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {components.map((comp) => {
            const compVersions = versions.filter(
              (cv) => cv.spec.componentRef.name === comp.metadata.name,
            );

            return (
              <Card key={comp.metadata.name} className="p-4">
                <div>
                  <CardTitle className="text-base">
                    {comp.spec.repository}
                  </CardTitle>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Registry:{" "}
                    <span className="font-mono">{comp.spec.registry}</span>
                    {" | "}
                    Scheme: {comp.spec.scheme}
                    {" | "}
                    Age: {formatAge(comp.metadata.creationTimestamp)}
                  </p>
                  {compVersions.length > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {compVersions.map((cv) => (
                        <Badge key={cv.metadata.name} variant="secondary">
                          {cv.spec.tag}
                        </Badge>
                      ))}
                    </div>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
