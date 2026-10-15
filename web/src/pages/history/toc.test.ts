import { describe, expect, it } from 'vitest';
import { extractToc } from './toc';

describe('extractToc', () => {
  it('extracts all six heading levels in source order', () => {
    expect(
      extractToc('# 页面标题\n## 第一节\n正文\n### 子节\n#### 四级\n##### 五级\n###### 六级'),
    ).toEqual([
      { id: '页面标题', level: 1, line: 0, title: '页面标题' },
      { id: '第一节', level: 2, line: 1, title: '第一节' },
      { id: '子节', level: 3, line: 3, title: '子节' },
      { id: '四级', level: 4, line: 4, title: '四级' },
      { id: '五级', level: 5, line: 5, title: '五级' },
      { id: '六级', level: 6, line: 6, title: '六级' },
    ]);
  });

  it('returns an empty table of contents without markdown headings', () => {
    expect(extractToc('')).toEqual([]);
    expect(extractToc('只有普通正文\n没有目录')).toEqual([]);
  });

  it('ignores invalid or empty headings, and code indentation', () => {
    expect(extractToc('####### 七级\n####没有空格\n######\n## ###\n    ## 代码')).toEqual([]);
  });

  it('accepts optional indentation and closing hashes without stripping C#', () => {
    expect(extractToc('   #### 四级标题 ####\r\n#####\tC#\r\n###### 六级')).toEqual([
      { id: '四级标题', level: 4, line: 0, title: '四级标题' },
      { id: 'c', level: 5, line: 1, title: 'C#' },
      { id: '六级', level: 6, line: 2, title: '六级' },
    ]);
  });

  it('does not extract headings inside backtick or tilde code fences', () => {
    expect(extractToc('```md\n# 示例\n```\n~~~~\n###### 示例\n~~~~\n#### 正文标题')).toEqual([
      { id: '正文标题', level: 4, line: 6, title: '正文标题' },
    ]);
  });

  it('avoids collisions between repeated headings and generated suffixes', () => {
    const toc = extractToc('# 学院\n#### 学院-2\n##### 学院\n###### 学院');
    expect(toc.map((item) => item.id)).toEqual(['学院', '学院-2', '学院-3', '学院-4']);
  });

  it('uses readable text for formatted headings and matches Markdown setext headings', () => {
    expect(extractToc('#### **学科**与`研究`\n\n学院沿革\n---')).toEqual([
      { id: '学科与研究', level: 4, line: 0, title: '学科与研究' },
      { id: '学院沿革', level: 2, line: 2, title: '学院沿革' },
    ]);
  });

  it('creates stable unique anchors for Chinese and duplicate titles', () => {
    expect(extractToc('## 学院沿革\n### 学院沿革\n## 2026 年工作计划')).toEqual([
      { id: '学院沿革', level: 2, line: 0, title: '学院沿革' },
      { id: '学院沿革-2', level: 3, line: 1, title: '学院沿革' },
      { id: '2026-年工作计划', level: 2, line: 2, title: '2026 年工作计划' },
    ]);
  });
});
