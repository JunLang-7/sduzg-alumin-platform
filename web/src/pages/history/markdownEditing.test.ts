import { describe, expect, it } from 'vitest';
import { applyMarkdownAction, resolveMarkdownShortcut } from './markdownEditing';

const press = (
  key: string,
  modifiers: Partial<Record<'ctrl' | 'meta' | 'alt' | 'shift', boolean>> = {},
) =>
  resolveMarkdownShortcut({
    key,
    code: /^[1-6]$/.test(key) ? `Digit${key}` : `Key${key.toUpperCase()}`,
    ctrlKey: modifiers.ctrl ?? false,
    metaKey: modifiers.meta ?? false,
    altKey: modifiers.alt ?? false,
    shiftKey: modifiers.shift ?? false,
  });

describe('Markdown keyboard shortcuts', () => {
  it('accepts Command on macOS and Control everywhere else for the same actions', () => {
    expect(press('b', { ctrl: true })).toBe('bold');
    expect(press('b', { meta: true })).toBe('bold');
    expect(press('i', { ctrl: true })).toBe('italic');
    expect(press('k', { meta: true })).toBe('link');
  });

  it('does not treat an unmodified or unknown key as a shortcut', () => {
    expect(press('b')).toBeUndefined();
    expect(press('z', { ctrl: true })).toBeUndefined();
  });

  it('keeps browser shortcuts starting with Shift untouched', () => {
    expect(press('b', { ctrl: true, shift: true })).toBeUndefined();
    expect(press('k', { ctrl: true, shift: true })).toBeUndefined();
  });

  it('switches heading levels with Alt plus a digit from one to six', () => {
    for (const level of ['1', '2', '3', '4', '5', '6'] as const) {
      expect(press(level, { ctrl: true, alt: true })).toBe(`heading-${level}`);
      expect(press(level, { meta: true, alt: true })).toBe(`heading-${level}`);
    }
    expect(press('7', { ctrl: true, alt: true })).toBeUndefined();
    expect(press('b', { ctrl: true, alt: true })).toBeUndefined();
  });

  it('reads the layout-independent digit code so macOS Option symbols still resolve', () => {
    expect(
      resolveMarkdownShortcut({
        key: '¡',
        code: 'Digit1',
        ctrlKey: false,
        metaKey: true,
        altKey: true,
        shiftKey: false,
      }),
    ).toBe('heading-1');
  });

  it('requires the exact heading combination and ignores an extra Shift', () => {
    expect(press('1', { ctrl: true, alt: true, shift: true })).toBeUndefined();
    expect(press('1', { meta: true, alt: true, shift: true })).toBeUndefined();
  });
});

describe('Markdown editing actions', () => {
  it('formats a Chinese selection without changing surrounding text and can toggle it off', () => {
    const result = applyMarkdownAction('学院简介正文', 2, 4, 'bold');
    expect(result).toEqual({ value: '学院**简介**正文', start: 4, end: 6 });
    expect(applyMarkdownAction(result.value, result.start, result.end, 'bold')).toEqual({
      value: '学院简介正文',
      start: 2,
      end: 4,
    });
  });

  it('inserts and selects a placeholder when there is no selected text', () => {
    expect(applyMarkdownAction('', 0, 0, 'italic')).toEqual({ value: '*文字*', start: 1, end: 3 });
  });

  it('can combine bold and italic without removing the existing formatting', () => {
    const italic = applyMarkdownAction('**学科简介**', 2, 6, 'italic');
    expect(italic.value).toBe('***学科简介***');
    expect(applyMarkdownAction(italic.value, italic.start, italic.end, 'italic').value).toBe(
      '**学科简介**',
    );
  });

  it('changes heading levels on the current line and toggles the same level', () => {
    const result = applyMarkdownAction('前文\n## 学科简介\n后文', 8, 8, 'heading-6');
    expect(result.value).toBe('前文\n###### 学科简介\n后文');
    const toggled = applyMarkdownAction(result.value, result.start, result.end, 'heading-6');
    expect(toggled.value).toBe('前文\n学科简介\n后文');
  });

  it('formats selected lines without including the next unselected line', () => {
    expect(applyMarkdownAction('第一项\n第二项\n后文', 0, 8, 'ordered').value).toBe(
      '1. 第一项\n2. 第二项\n后文',
    );
    const list = '1. 第一项\n2. 第二项';
    expect(applyMarkdownAction(list, 0, list.length, 'ordered').value).toBe('第一项\n第二项');
  });

  it('selects the link destination for immediate editing', () => {
    const result = applyMarkdownAction('学院网站', 0, 4, 'link');
    expect(result.value).toBe('[学院网站](<https://example.com>)');
    expect(result.value.slice(result.start, result.end)).toBe('https://example.com');
  });

  it('escapes link labels and keeps only the URL selected', () => {
    const label = '学院]网站[资料\\';
    const result = applyMarkdownAction(label, 0, label.length, 'link');
    expect(result.value).toBe('[学院\\]网站\\[资料\\\\](<https://example.com>)');
    expect(result.value.slice(result.start, result.end)).toBe('https://example.com');
  });

  it('supports quotes, tasks, unordered lists, strikeout and inline code', () => {
    expect(applyMarkdownAction('正文', 0, 2, 'quote').value).toBe('> 正文');
    expect(applyMarkdownAction('正文', 0, 2, 'task').value).toBe('- [ ] 正文');
    expect(applyMarkdownAction('正文', 0, 2, 'unordered').value).toBe('- 正文');
    expect(applyMarkdownAction('正文', 0, 2, 'strike').value).toBe('~~正文~~');
    expect(applyMarkdownAction('正文', 0, 2, 'code').value).toBe('`正文`');
  });

  it('uses a longer code fence when the selected code already includes backticks', () => {
    const code = '```\n代码\n```';
    const result = applyMarkdownAction(code, 0, code.length, 'code-block');
    expect(result.value).toBe('````\n```\n代码\n```\n````\n');
    expect(result.value.slice(result.start, result.end)).toBe(code);
  });
});
