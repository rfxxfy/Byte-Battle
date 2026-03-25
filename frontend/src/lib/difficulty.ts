import type { Problem } from '@/api/problems'

export const difficultyLabel: Record<Problem['difficulty'], string> = {
  easy: 'Лёгкая',
  medium: 'Средняя',
  hard: 'Сложная',
}

export const difficultyClass: Record<Problem['difficulty'], string> = {
  easy: 'text-green-400 bg-green-400/10 border-green-400/20',
  medium: 'text-yellow-400 bg-yellow-400/10 border-yellow-400/20',
  hard: 'text-red-400 bg-red-400/10 border-red-400/20',
}
