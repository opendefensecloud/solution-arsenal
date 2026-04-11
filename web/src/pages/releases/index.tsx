import { useQuery } from "@tanstack/react-query";
import { releaseQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";

const namespace = "default";

export function ReleasesPage() {
  useSSE(namespace);

  const { data, isLoading } = useQuery(releaseQueries.list(namespace));

  if (isLoading) {
    return <div className="text-muted-foreground">Loading releases...</div>;
  }

  const releases = data?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">Releases</h1>
        <span className="text-sm text-muted-foreground">
          {releases.length} release{releases.length !== 1 ? "s" : ""}
        </span>
      </div>

      {releases.length === 0 ? (
        <Card>
          <CardContent>
            <p className="text-center text-muted-foreground">
              No releases found
            </p>
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
