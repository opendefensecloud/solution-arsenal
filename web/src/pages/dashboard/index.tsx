import { useQuery } from "@tanstack/react-query";
import {
  targetQueries,
  releaseQueries,
  componentQueries,
  renderTaskQueries,
} from "@/api/queries";
import { Card, CardTitle, CardContent } from "@/components/ui/card";
import { useSSE } from "@/hooks/useSSE";
import { useNamespace } from "@/hooks/useNamespace";
import { Server, Package, Boxes, Loader } from "lucide-react";

export function DashboardPage() {
  const { namespace } = useNamespace();
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
      error: targets.isError,
      color: "text-blue-600 dark:text-blue-400",
      bg: "bg-blue-50 dark:bg-blue-500/10",
    },
    {
      label: "Releases",
      value: releases.data?.items.length ?? 0,
      icon: Package,
      loading: releases.isLoading,
      error: releases.isError,
      color: "text-primary",
      bg: "bg-primary/10",
    },
    {
      label: "Components",
      value: components.data?.items.length ?? 0,
      icon: Boxes,
      loading: components.isLoading,
      error: components.isError,
      color: "text-emerald-600 dark:text-emerald-400",
      bg: "bg-emerald-50 dark:bg-emerald-500/10",
    },
    {
      label: "Active Renders",
      value: renderTasks.data?.items.length ?? 0,
      icon: Loader,
      loading: renderTasks.isLoading,
      error: renderTasks.isError,
      color: "text-violet-600 dark:text-violet-400",
      bg: "bg-violet-50 dark:bg-violet-500/10",
    },
  ];

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-foreground">Dashboard</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          namespace <span className="font-mono">{namespace ?? "all"}</span>
        </p>
      </div>

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map(({ label, value, icon: Icon, loading, error, color, bg }) => (
          <Card key={label} className="p-5">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  {label}
                </p>
                <CardContent>
                  {error ? (
                    <p className="mt-1 text-base font-medium text-destructive">
                      Failed to load
                    </p>
                  ) : (
                    <p className="mt-1 text-3xl font-bold text-foreground">
                      {loading ? "-" : value}
                    </p>
                  )}
                </CardContent>
              </div>
              <div className={`rounded-lg ${bg} p-3`}>
                <Icon className={`h-5 w-5 ${color}`} />
              </div>
            </div>
          </Card>
        ))}
      </div>

      {/* Recent render tasks */}
      {renderTasks.data && renderTasks.data.items.length > 0 && (
        <Card className="mt-8">
          <CardTitle>Recent Render Tasks</CardTitle>
          <CardContent className="mt-4">
            <div className="divide-y divide-border">
              {renderTasks.data.items.slice(0, 10).map((rt) => (
                <div
                  key={`${rt.metadata.namespace ?? "unknown"}/${rt.metadata.name}`}
                  className="flex items-center justify-between py-3"
                >
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
