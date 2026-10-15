export type MarkdownAction =
  | 'bold'
  | 'italic'
  | 'strike'
  | 'link'
  | 'code'
  | 'code-block'
  | 'quote'
  | 'ordered'
  | 'unordered'
  | 'task'
  | `heading-${1 | 2 | 3 | 4 | 5 | 6}`;

export interface MarkdownEdit {
  value: string;
  start: number;
  end: number;
}

export function applyMarkdownAction(
  value: string,
  start: number,
  end: number,
  action: MarkdownAction,
): MarkdownEdit {
  const selected = value.slice(start, end);
  const replace = (
    from: number,
    to: number,
    text: string,
    selectionStart: number,
    selectionEnd: number,
  ) => ({
    value: value.slice(0, from) + text + value.slice(to),
    start: from + selectionStart,
    end: from + selectionEnd,
  });
  if (action === 'bold' || action === 'italic' || action === 'strike' || action === 'code') {
    const mark = {
      bold: '**',
      italic: '*',
      strike: '~~',
      code: selected.includes('`') ? '``' : '`',
    }[action];
    const text = selected || (action === 'code' ? '代码' : '文字');
    let left = 0;
    let right = 0;
    while (value[start - left - 1] === mark[0]) left++;
    while (value[end + right] === mark[0]) right++;
    const wrapped =
      action === 'italic'
        ? left % 2 === 1 && right % 2 === 1
        : left >= mark.length && right >= mark.length;
    if (wrapped) {
      return replace(start - mark.length, end + mark.length, selected, 0, selected.length);
    }
    return replace(start, end, mark + text + mark, mark.length, mark.length + text.length);
  }
  if (action === 'link') {
    const label = selected || '链接文字';
    const url = 'https://example.com';
    return replace(
      start,
      end,
      `[${label}](${url})`,
      label.length + 3,
      label.length + 3 + url.length,
    );
  }
  if (action === 'code-block') {
    const text = selected || '代码';
    const fence = '`'.repeat(
      Math.max(3, ...(text.match(/`+/g) ?? []).map((run) => run.length + 1)),
    );
    const prefix = (start > 0 ? '\n\n' : '') + fence + '\n';
    return replace(
      start,
      end,
      `${prefix}${text}\n${fence}\n`,
      prefix.length,
      prefix.length + text.length,
    );
  }
  const from = start === 0 ? 0 : value.lastIndexOf('\n', start - 1) + 1;
  const lastSelected = end > start && value[end - 1] === '\n' ? end - 1 : end;
  const newline = value.indexOf('\n', lastSelected);
  const to = newline < 0 ? value.length : newline;
  const lines = value.slice(from, to).split('\n');
  const headingLevel = action.startsWith('heading-') ? Number(action.slice(-1)) : undefined;
  const marker = headingLevel
    ? '#'.repeat(headingLevel) + ' '
    : { quote: '> ', ordered: '1. ', unordered: '- ', task: '- [ ] ' }[
        action as 'quote' | 'ordered' | 'unordered' | 'task'
      ];
  const toggledOff = lines.every((line, index) =>
    line.startsWith(action === 'ordered' ? `${index + 1}. ` : marker),
  );
  const text = lines
    .map((line, index) => {
      if (toggledOff)
        return line.slice(action === 'ordered' ? String(index + 1).length + 2 : marker.length);
      const plain = headingLevel ? line.replace(/^ {0,3}#{1,6}[\t ]+/, '') : line;
      return (action === 'ordered' ? `${index + 1}. ` : marker) + plain;
    })
    .join('\n');
  const caret = Math.max(0, start - from + text.length - (to - from));
  return replace(
    from,
    to,
    text,
    start === end ? Math.min(caret, text.length) : 0,
    start === end ? Math.min(caret, text.length) : text.length,
  );
}
