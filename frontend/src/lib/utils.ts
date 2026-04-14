import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function displayName(name: string | null | undefined, id: string): string {
  return name ?? id.slice(0, 8)
}

export function formatTimer(minutes: number | null | undefined): string {
  if (!minutes) return 'Без таймера'
  if (minutes < 60) return `${minutes} мин`
  return `${minutes / 60} ч`
}

export function pluralize(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few
  return many
}
