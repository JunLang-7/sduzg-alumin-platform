import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { HistoryContent } from './HistoryContent';
import { extractToc } from './toc';

describe('HistoryContent', () => {
  it('renders all six heading levels with the exact anchors used by the TOC', () => {
    const content = '# 一级\n## 二级\n### 三级\n#### 四级\n##### 五级\n###### 六级\n普通正文';
    const toc = extractToc(content);
    const html = renderToStaticMarkup(<HistoryContent content={content} toc={toc} />);

    for (const heading of toc) {
      expect(html).toContain(
        `<h${heading.level} class="history-page__heading" data-level="${heading.level}" id="${heading.id}">${heading.title}</h${heading.level}>`,
      );
    }
    expect(html).toContain('<p>普通正文</p>');
  });

  it('renders skipped levels and duplicate headings without losing anchor targets', () => {
    const content = '## 简介\n###### 简介\n####### 普通文本';
    const html = renderToStaticMarkup(
      <HistoryContent content={content} toc={extractToc(content)} />,
    );
    expect(html).toContain('id="简介"');
    expect(html).toContain('id="简介-2"');
    expect(html).toContain('<p>####### 普通文本</p>');
  });

  it('renders toolbar formatting, tasks and links with the same Markdown renderer', () => {
    const content =
      '#### **学科简介**\n\n**加粗** *斜体* ~~删除~~ `代码`\n\n> 引用\n\n1. 第一项\n2. 第二项\n\n- [ ] 待办\n\n[学院网站](https://example.com)';
    const html = renderToStaticMarkup(<HistoryContent content={content} />);
    expect(html).toContain('id="学科简介"><strong>学科简介</strong></h4>');
    for (const fragment of [
      '<strong>加粗</strong>',
      '<em>斜体</em>',
      '<del>删除</del>',
      '<code>代码</code>',
      '<blockquote>',
      '<ol>',
      'type="checkbox"',
      'href="https://example.com"',
    ]) {
      expect(html).toContain(fragment);
    }
    expect(extractToc(content)[0].title).toBe('学科简介');
  });

  it('does not execute raw HTML or allow script URLs', () => {
    const content =
      '<script>alert(1)</script>\n\n[危险](javascript:alert(1))\n\n<img src=x onerror="alert(1)">';
    const html = renderToStaticMarkup(<HistoryContent content={content} />);
    expect(html).not.toContain('<script');
    expect(html).not.toContain('javascript:');
    expect(html).not.toContain('onerror');
  });
});
