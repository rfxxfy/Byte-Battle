const messages: Record<string, string> = {
  // Auth
  INVALID_EMAIL: 'Некорректный email',
  INVALID_CODE: 'Неверный или истёкший код',
  TOO_MANY_ATTEMPTS: 'Слишком много попыток, попробуй позже',
  CODE_RECENTLY_SENT: 'Код уже был отправлен, подожди немного',
  USER_NOT_FOUND: 'Пользователь не найден',
  SESSION_NOT_FOUND: 'Сессия не найдена',
  SESSION_EXPIRED: 'Сессия истекла, войди снова',
  INVALID_TOKEN: 'Неверный токен',

  // Games
  GAME_NOT_FOUND: 'Игра не найдена',
  NOT_ENOUGH_PLAYERS: 'Недостаточно игроков для старта',
  ALREADY_PARTICIPANT: 'Ты уже участник этой игры',
  GAME_ALREADY_STARTED: 'Игра уже началась или завершена',
  GAME_NOT_IN_PROGRESS: 'Игра не в процессе',
  INVALID_WINNER: 'Некорректный победитель',
  NOT_GAME_CREATOR: 'Только создатель может начать игру',
  CANNOT_CANCEL_FINISHED_GAME: 'Нельзя отменить завершённую игру',
  GAME_ALREADY_CANCELLED: 'Игра уже отменена',
  GAME_NOT_FINISHED: 'Решения доступны только после завершения игры',

  // Problems
  PROBLEM_NOT_FOUND: 'Задача не найдена',
  NOT_PROBLEM_OWNER: 'Вы не являетесь владельцем этой задачи',
  ARCHIVE_INVALID: 'Ошибка формата архива',
  VERSION_LIMIT_REACHED: 'Достигнут лимит версий задачи (максимум 10)',
  PROBLEM_LIMIT_REACHED: 'Достигнут лимит задач (максимум 20)',
  EXECUTOR_NOT_READY: 'Система выполнения кода временно недоступна',

  // Generic
  VALIDATION_ERROR: 'Ошибка валидации',
  INTERNAL_ERROR: 'Внутренняя ошибка сервера',
}

export function errorMessage(code: string, fallback?: string): string {
  return messages[code] ?? fallback ?? 'Что-то пошло не так'
}
