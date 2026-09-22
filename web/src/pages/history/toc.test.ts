import { describe, expect, it } from 'vitest';
import { extractToc } from './toc';

describe('extractToc', () => {
  it('extracts h2 and h3 headings in source order', () => {
    expect(extractToc('# 页面标题\n## 第一节\n正文\n### 子节\n#### 忽略')).toEqual([
      { id: '第一节', level: 2, line: 1, title: '第一节' },
      { id: '子节', level: 3, line: 3, title: '子节' },
    ]);
  });

  it('returns an empty table of contents without markdown headings', () => {
    expect(extractToc('只有普通正文\n没有目录')).toEqual([]);
  });

  it('creates stable unique anchors for Chinese and duplicate titles', () => {
    expect(extractToc('## 学院沿革\n### 学院沿革\n## 2026 年工作计划')).toEqual([
      { id: '学院沿革', level: 2, line: 0, title: '学院沿革' },
      { id: '学院沿革-2', level: 3, line: 1, title: '学院沿革' },
      { id: '2026-年工作计划', level: 2, line: 2, title: '2026 年工作计划' },
    ]);
  });
});
