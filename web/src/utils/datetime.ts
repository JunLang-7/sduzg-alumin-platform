const FALLBACK = '—';

export interface FormatDateTimeOptions {
  seconds?: boolean;
  timeZone?: string;
  fallback?: string;
}

type DateInput = string | number | Date | null | undefined;

function parseDate(value: DateInput): Date | null {
  if (value === null || value === undefined) return null;
  if (typeof value === 'string' && value.trim() === '') return null;
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

// 空值与非法输入按原值回退：空值用 fallback，非法字符串保留原样，避免页面显示乱码时间。
function fallbackValue(value: DateInput, fallback = FALLBACK): string {
  if (typeof value === 'string' && value.trim() !== '') return value;
  if (typeof value === 'number') return String(value);
  return fallback;
}

interface FormatPartsOptions {
  timeZone?: string;
  seconds?: boolean;
  includeTime: boolean;
}

// 固定输出 YYYY-MM-DD[ HH:mm[:ss]]，不依赖 zh-CN 的斜杠或全角分隔符。
function formatParts(date: Date, options: FormatPartsOptions): string {
  const parts = new Intl.DateTimeFormat('zh-CN', {
    ...(options.timeZone ? { timeZone: options.timeZone } : {}),
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    ...(options.includeTime
      ? {
          hour: '2-digit',
          minute: '2-digit',
          ...(options.seconds ? { second: '2-digit' } : {}),
          hourCycle: 'h23',
        }
      : {}),
  }).formatToParts(date);
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? '';
  const datePart = `${pick('year')}-${pick('month')}-${pick('day')}`;
  if (!options.includeTime) return datePart;
  const timePart = `${pick('hour')}:${pick('minute')}${
    options.seconds ? `:${pick('second')}` : ''
  }`;
  return `${datePart} ${timePart}`;
}

// 默认使用浏览器本地时区；需要固定时区的场景显式传 timeZone。
export function formatDateTime(value: DateInput, options: FormatDateTimeOptions = {}): string {
  const date = parseDate(value);
  if (!date) return fallbackValue(value, options.fallback);
  return formatParts(date, {
    includeTime: true,
    seconds: options.seconds,
    timeZone: options.timeZone,
  });
}

export function formatDate(
  value: DateInput,
  options: Omit<FormatDateTimeOptions, 'seconds'> = {},
): string {
  const date = parseDate(value);
  if (!date) return fallbackValue(value, options.fallback);
  return formatParts(date, { includeTime: false, timeZone: options.timeZone });
}
