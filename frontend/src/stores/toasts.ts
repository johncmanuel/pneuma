import { writable } from "svelte/store";

export interface Toast {
  id: number;
  message: string;
  type: "info" | "warning" | "error" | "success";
  duration?: number; // ms; undefined = sticky (manual dismiss)
}

let nextId = 1;

export const toasts = writable<Toast[]>([]);

export function addToast(
  message: string,
  type: Toast["type"] = "info",
  duration = 6000
) {
  const id = nextId++;
  toasts.update((t) => [...t, { id, message, type, duration }]);
  if (duration > 0) {
    setTimeout(() => dismissToast(id), duration);
  }
  return id;
}

export function dismissToast(id: number) {
  toasts.update((t) => t.filter((x) => x.id !== id));
}
