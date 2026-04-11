import type { Condition } from "@/api/types";
import { Badge } from "./badge";
import { getCondition } from "@/lib/utils";

interface StatusBadgeProps {
  conditions?: Condition[];
  type?: string;
}

export function StatusBadge({
  conditions,
  type = "Ready",
}: StatusBadgeProps) {
  const condition = getCondition(conditions, type);

  if (!condition) {
    return <Badge variant="secondary">Unknown</Badge>;
  }

  switch (condition.status) {
    case "True":
      return <Badge variant="success">{condition.reason || "Ready"}</Badge>;
    case "False":
      return (
        <Badge variant="destructive">{condition.reason || "Not Ready"}</Badge>
      );
    default:
      return <Badge variant="warning">{condition.reason || "Pending"}</Badge>;
  }
}
