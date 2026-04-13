import { useQuery } from "@tanstack/react-query";
import { releaseQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";
import { Package } from "lucide-react";

const namespace = "default";

export function ReleasesPage() {
  useSSE(namespace);

  const { data, isLoading } = useQuery(releaseQueries.list(namespace));

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Package className="h-4 w-4 animate-pulse" />
        Loading releases...
      </div>
    );
  }

  const releases = data?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">Releases</h1>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {releases.length} release{releases.length !== 1 ? "s" : ""}
        </span>
      </div>

      {releases.length === 0 ? (
        <Card className="py-12">
          <CardContent>
            <div className="text-center">
              <Package className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
              <p className="text-muted-foreground">No releases found</p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {releases.map((release) => (
            <Card key={release.metadata.name} className="p-4">
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-base">
                    {release.metadata.name}
                  </CardTitle>
                  <p className="mt-1 text-xs text-muted-foreground">
                    ComponentVersion:{" "}
                    <span className="font-mono">
                      {release.spec.componentVersionRef.name}
                    </span>
                    {" | "}
                    Age: {formatAge(release.metadata.creationTimestamp)}
                  </p>
                </div>
                <StatusBadge
                  conditions={release.status?.conditions}
                  type="ComponentVersionResolved"
                />
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
