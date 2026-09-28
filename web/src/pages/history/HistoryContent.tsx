import type { TocItem } from './toc';

export function HistoryContent({ content, toc }: { content: string; toc: TocItem[] }) {
  const headings = new Map(toc.map((item) => [item.line, item]));
  return (
    <div className="history-page__body">
      {content.split(/\r?\n/).map((line, index) => {
        const heading = headings.get(index);
        if (heading) {
          const Heading = heading.level === 2 ? 'h3' : 'h4';
          return (
            <Heading id={heading.id} key={heading.id}>
              {heading.title}
            </Heading>
          );
        }
        return line ? <p key={`${index}-${line}`}>{line}</p> : <br key={index} />;
      })}
    </div>
  );
}
