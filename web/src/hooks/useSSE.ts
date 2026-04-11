import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { ResourceEvent } from "@/api/types";

export function useSSE(namespace: string) {
  const queryClient = useQueryClient();

  useEffect(() => {
    const source = new EventSource(`/api/namespaces/${namespace}/events`);

    source.onmessage = (event) => {
      try {
        const data: ResourceEvent = JSON.parse(event.data);
        // Invalidate the relevant query when a resource changes
        queryClient.invalidateQueries({
          queryKey: [data.resource, data.namespace],
        });
      } catch {
        // ignore parse errors
      }
    };

    source.onerror = () => {
      // EventSource auto-reconnects
    };

    return () => source.close();
  }, [namespace, queryClient]);
}
