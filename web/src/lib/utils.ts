import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import type { Condition } from "@/api/types";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function getCondition(
  conditions: Condition[] | undefined,
  type: string,
): Condition | undefined {
  return conditions?.find((c) => c.type === type);
}

export function isReady(conditions: Condition[] | undefined): boolean {
  const ready = getCondition(conditions, "Ready");
  return ready?.status === "True";
}

export function formatAge(timestamp: string): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}
