import { useQuery } from "@tanstack/react-query";
import { profileQueries } from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-badge";
import { useSSE } from "@/hooks/useSSE";
import { formatAge } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Users } from "lucide-react";

const namespace = "default";

export function ProfilesPage() {
  useSSE(namespace);

  const { data, isLoading } = useQuery(profileQueries.list(namespace));

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Users className="h-4 w-4 animate-pulse" />
        Loading profiles...
      </div>
    );
  }

  const profiles = data?.items ?? [];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">Profiles</h1>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {profiles.length} profile{profiles.length !== 1 ? "s" : ""}
        </span>
      </div>

      {profiles.length === 0 ? (
        <Card className="py-12">
          <CardContent>
            <div className="text-center">
              <Users className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />
              <p className="text-muted-foreground">No profiles found</p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {profiles.map((profile) => (
            <Card key={profile.metadata.name} className="p-4">
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-base">
                    {profile.metadata.name}
                  </CardTitle>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Release:{" "}
                    <span className="font-mono">
                      {profile.spec.releaseRef.name}
                    </span>
                    {" | "}
                    Age: {formatAge(profile.metadata.creationTimestamp)}
                    {profile.status?.matchedTargets !== undefined && (
                      <>
                        {" | "}
                        {profile.status.matchedTargets} target
                        {profile.status.matchedTargets !== 1 ? "s" : ""}{" "}
                        matched
                      </>
                    )}
                  </p>
                  {profile.spec.targetSelector.matchLabels && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {Object.entries(
                        profile.spec.targetSelector.matchLabels,
                      ).map(([k, v]) => (
                        <Badge key={k} variant="secondary">
                          {k}={v}
                        </Badge>
                      ))}
                    </div>
                  )}
                </div>
                <StatusBadge conditions={profile.status?.conditions} />
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
