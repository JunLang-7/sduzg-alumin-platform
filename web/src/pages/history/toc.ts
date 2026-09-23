export interface TocItem {
  id: string;
  level: 2 | 3;
  line: number;
  title: string;
}

const headingPattern = /^(#{2,3})\s+(.+?)\s*#*\s*$/;

const anchorBase = (title: string) => {
  const normalized = title
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\w\-\u4e00-\u9fff]/g, '');
  return normalized || 'section';
};

// Extract only Markdown h2/h3 lines. Rendering can remain plain text while
// these stable ids already provide navigation for a later Markdown renderer.
export function extractToc(content: string): TocItem[] {
  const usedIDs = new Map<string, number>();
  const result: TocItem[] = [];

  content.split(/\r?\n/).forEach((line, index) => {
    const match = line.match(headingPattern);
    if (!match) return;

    const title = match[2].trim();
    const base = anchorBase(title);
    const count = (usedIDs.get(base) ?? 0) + 1;
    usedIDs.set(base, count);
    result.push({
      id: count === 1 ? base : `${base}-${count}`,
      level: match[1].length as 2 | 3,
      line: index,
      title,
    });
  });

  return result;
}
