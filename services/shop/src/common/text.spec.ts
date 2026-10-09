import { ApiError } from './api-error';
import { cleanLine, cleanText } from './text';

describe('text cleaning', () => {
  it('collapses whitespace in a line', () => {
    expect(cleanLine('  Pagnes \n  Awa ', 60, 'code', 'name')).toBe('Pagnes Awa');
  });

  it.each(['', '   ', 'a'.repeat(61), 'a‮b'])('rejects the line %p', (raw) => {
    expect(() => cleanLine(raw, 60, 'name_invalid', 'name')).toThrow(ApiError);
  });

  it('keeps line breaks in a text and clears a blank one', () => {
    expect(cleanText(' Wax\r\nBazin ', 100, 3, 'code', 'description')).toBe('Wax\nBazin');
    expect(cleanText('   ', 100, 3, 'code', 'description')).toBeNull();
    expect(cleanText(null, 100, 3, 'code', 'description')).toBeNull();
  });

  it.each(['a\nb\nc\nd', 'x'.repeat(101), 'a​b'])('rejects the text %p', (raw) => {
    expect(() => cleanText(raw, 100, 3, 'code', 'description')).toThrow(ApiError);
  });
});
