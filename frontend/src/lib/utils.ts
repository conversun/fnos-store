import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`;
  return `${(bytes / 1073741824).toFixed(1)} GB`;
}

export function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`;
}

export function formatProgress(downloaded: number, total: number): string {
  const units = ['B', 'KB', 'MB', 'GB'];
  const thresholds = [1, 1024, 1048576, 1073741824];
  let idx = 0;
  for (let i = thresholds.length - 1; i >= 0; i--) {
    if (total >= thresholds[i]) { idx = i; break; }
  }
  const d = thresholds[idx];
  const fmt = (v: number) => d === 1 ? String(v) : (v / d).toFixed(1);
  return `${fmt(downloaded)}/${fmt(total)} ${units[idx]}`;
}

export function formatCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 10000) return `${(n / 1000).toFixed(1)}k`;
  if (n < 1000000) return `${Math.round(n / 1000)}k`;
  return `${(n / 1000000).toFixed(1)}M`;
}
