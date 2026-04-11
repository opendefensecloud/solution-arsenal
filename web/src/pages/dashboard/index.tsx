import { useQuery } from "@tanstack/react-query";
import {
  targetQueries,
  releaseQueries,
  componentQueries,
  renderTaskQueries,
} from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { useSSE } from "@/hooks/useSSE";
import { Server, Package, Boxes, Loader } from "lucide-react";

const namespace = "default"; // TODO: namespace selector

export function DashboardPage() {
  useSSE(namespace);

  const targets = useQuery(targetQueries.list(namespace));
  const releases = useQuery(releaseQueries.list(namespace));
  const components = useQuery(componentQueries.list(namespace));
  const renderTasks = useQuery(renderTaskQueries.list(namespace));

  const stats = [
    {
      label: "Targets",
      value: targets.data?.items.length ?? 0,
      icon: Server,
      loading: targets.isLoading,
    },
    {
      label: "Releases",
      value: releases.data?.items.length ?? 0,
      icon: Package,
      loading: releases.isLoading,
    },
    {
      label: "Components",
      value: components.data?.items.length ?? 0,
      icon: Boxes,
      loading: components.isLoading,
    },
    {
      label: "Active Renders",
      value: renderTasks.data?.items.length ?? 0,
      icon: Loader,
      loading: renderTasks.isLoading,
    },
  ];

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-foreground">Dashboard</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map(({ label, value, icon: Icon, loading }) => (
          <Card key={label}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">{label}</p>
                <CardContent>
                  {loading ? (
                    <p className="text-2xl font-bold text-muted-foreground">
                      -
                    </p>
                  ) : (
                    <p className="text-2xl font-bold text-foreground">
                      {value}
                    </p>
                  )}
                </CardContent>
              </div>
              <Icon className="h-8 w-8 text-muted-foreground/50" />
            </div>
          </Card>
        ))}
      </div>

      {/* Recent render tasks */}
      {renderTasks.data && renderTasks.data.items.length > 0 && (
        <Card className="mt-6">
          <CardTitle>Recent Render Tasks</CardTitle>
          <CardContent>
            <div className="divide-y">
              {renderTasks.data.items.slice(0, 10).map((rt) => (
                <div key={rt.metadata.name} className="flex items-center justify-between py-3">
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {rt.metadata.name}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {rt.spec.type}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
