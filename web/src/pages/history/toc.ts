import type { Nodes } from 'mdast';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import { unified } from 'unified';

export type HeadingLevel = 1 | 2 | 3 | 4 | 5 | 6;

export interface TocItem {
  id: string;
  level: HeadingLevel;
  line: number;
  title: string;
}

const parser = unified().use(remarkParse).use(remarkGfm);

const headingText = (node: Nodes): string => {
  if (node.type === 'text' || node.type === 'inlineCode') return node.value;
  if (node.type === 'image' || node.type === 'imageReference') return node.alt ?? '';
  return 'children' in node ? node.children.map(headingText).join('') : '';
};

const anchorBase = (title: string) => {
  const normalized = title
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^\w\-\u4e00-\u9fff]/g, '');
  return normalized || 'section';
};

// Use the same heading levels and stable anchors for content and navigation.
export function extractToc(content: string): TocItem[] {
  const usedIDs = new Set<string>();
  const result: TocItem[] = [];
  const visit = (node: Nodes) => {
    if (node.type === 'heading') {
      const title = headingText(node).trim();
      if (!title) return;
      const base = anchorBase(title);
      let id = base;
      let count = 2;
      while (usedIDs.has(id)) id = `${base}-${count++}`;
      usedIDs.add(id);
      result.push({ id, level: node.depth, line: node.position!.start.line - 1, title });
    }
    if ('children' in node) node.children.forEach(visit);
  };
  visit(parser.parse(content));

  return result;
}
