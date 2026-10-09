import { ApiError } from './api-error';

// Control characters, bidi overrides and zero-width characters are used to spoof names.
// eslint-disable-next-line no-control-regex
const FORBIDDEN_CHARS = /[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u202a-\u202e\u2066-\u2069\ufeff]/;

// Postgres VARCHAR(n) counts code points, not UTF-16 units or graphemes.
export function codePoints(value: string): number {
  // eslint-disable-next-line @typescript-eslint/no-misused-spread
  return [...value].length;
}

/** A required one-line text: trimmed, inner whitespace collapsed, 1 to max code points. */
export function cleanLine(raw: string, max: number, code: string, field: string): string {
  const value = raw.normalize('NFC').trim().replace(/\s+/g, ' ');
  if (value === '' || codePoints(value) > max || FORBIDDEN_CHARS.test(value)) {
    throw new ApiError(422, code, `${field} must be 1 to ${max} characters`);
  }
  return value;
}

/** An optional multi-line text: null or blank clears it. */
export function cleanText(
  raw: string | null,
  max: number,
  maxLines: number,
  code: string,
  field: string,
): string | null {
  if (raw === null) {
    return null;
  }
  const value = raw.normalize('NFC').replace(/\r\n?/g, '\n').trim();
  if (value === '') {
    return null;
  }
  const lines = value.split('\n');
  if (
    codePoints(value) > max ||
    lines.length > maxLines ||
    lines.some((line) => FORBIDDEN_CHARS.test(line))
  ) {
    throw new ApiError(
      422,
      code,
      `${field} must be at most ${max} characters and ${maxLines} lines`,
    );
  }
  return value;
}
