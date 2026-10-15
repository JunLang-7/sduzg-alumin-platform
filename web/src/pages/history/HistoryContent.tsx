import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { extractToc, type TocItem } from './toc';
import './history-content.css';

const headingTags = ['h1', 'h2', 'h3', 'h4', 'h5', 'h6'] as const;

export function HistoryContent({
  content,
  toc = extractToc(content),
}: {
  content: string;
  toc?: TocItem[];
}) {
  const headings = new Map(toc.map((item) => [item.line, item]));
  const components: Components = {};
  for (const tag of headingTags) {
    components[tag] = ({ node, children }) => {
      const heading = headings.get((node?.position?.start.line ?? 0) - 1);
      const Heading = tag;
      return (
        <Heading className="history-page__heading" data-level={Number(tag[1])} id={heading?.id}>
          {children}
        </Heading>
      );
    };
  }
  return (
    <div className="history-page__body">
      <Markdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          ...components,
          a: ({ children, href }) => (
            <a href={href} target="_blank" rel="noopener noreferrer">
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div className="history-content__table">
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {content}
      </Markdown>
    </div>
  );
}
