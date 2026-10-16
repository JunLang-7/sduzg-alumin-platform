import {
  AlignLeftOutlined,
  BoldOutlined,
  CheckSquareOutlined,
  CodeOutlined,
  DownOutlined,
  FontSizeOutlined,
  ItalicOutlined,
  LinkOutlined,
  OrderedListOutlined,
  StrikethroughOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import { Button, Dropdown, Empty, Input, Tooltip } from 'antd';
import type { TextAreaRef } from 'antd/es/input/TextArea';
import { flushSync } from 'react-dom';
import { useId, useRef, useState, type AriaAttributes, type KeyboardEvent } from 'react';
import { HistoryContent } from './HistoryContent';
import {
  applyMarkdownAction,
  resolveMarkdownShortcut,
  type MarkdownAction,
} from './markdownEditing';
import './markdown-editor.css';

interface MarkdownEditorProps extends AriaAttributes {
  id?: string;
  value?: string;
  onChange?: (value: string) => void;
  onBlur?: () => void;
}

const tools = [
  { action: 'bold', label: '加粗', hint: 'Ctrl/⌘ + B', icon: <BoldOutlined /> },
  { action: 'italic', label: '斜体', hint: 'Ctrl/⌘ + I', icon: <ItalicOutlined /> },
  { action: 'strike', label: '删除线', icon: <StrikethroughOutlined /> },
  { action: 'quote', label: '引用', icon: <AlignLeftOutlined /> },
  { action: 'code', label: '行内代码', icon: <CodeOutlined /> },
  { action: 'code-block', label: '代码块', icon: <span aria-hidden="true">{'{ }'}</span> },
  { action: 'link', label: '链接', hint: 'Ctrl/⌘ + K', icon: <LinkOutlined /> },
  { action: 'ordered', label: '有序列表', icon: <OrderedListOutlined /> },
  { action: 'unordered', label: '无序列表', icon: <UnorderedListOutlined /> },
  { action: 'task', label: '任务列表', icon: <CheckSquareOutlined /> },
] satisfies { action: MarkdownAction; label: string; hint?: string; icon: React.ReactNode }[];

export function MarkdownEditor({ value = '', onChange, id, ...inputProps }: MarkdownEditorProps) {
  const generatedID = useId();
  const editorID = id || generatedID;
  const [tab, setTab] = useState<'write' | 'preview'>('write');
  const input = useRef<TextAreaRef>(null);
  const selection = useRef({ start: 0, end: 0 });
  const rememberSelection = () => {
    const textarea = input.current?.resizableTextArea?.textArea;
    if (textarea)
      selection.current = { start: textarea.selectionStart, end: textarea.selectionEnd };
  };
  const apply = (action: MarkdownAction) => {
    const textarea = input.current?.resizableTextArea?.textArea;
    const { start, end } = textarea
      ? { start: textarea.selectionStart, end: textarea.selectionEnd }
      : selection.current;
    // Read the live DOM for commands dispatched before a controlled render commits.
    const edit = applyMarkdownAction(textarea?.value ?? value, start, end, action);
    selection.current = { start: edit.start, end: edit.end };
    if (textarea) {
      // Synchronize before notifying React: the next command must not wait for a frame.
      textarea.value = edit.value;
      textarea.focus({ preventScroll: true });
      textarea.setSelectionRange(edit.start, edit.end);
    }
    // Commit the controlled field too, so rapid toggles cannot use a stale Form callback.
    if (onChange) flushSync(() => onChange(edit.value));
  };
  const keyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.nativeEvent.isComposing) return;
    const action = resolveMarkdownShortcut(event.nativeEvent);
    if (action) {
      event.preventDefault();
      apply(action);
    }
  };
  return (
    <div className="markdown-editor">
      <div className="markdown-editor__header">
        <div className="markdown-editor__tabs" role="tablist" aria-label="正文编辑模式">
          {(['write', 'preview'] as const).map((mode) => (
            <button
              key={mode}
              id={`${editorID}-${mode}-tab`}
              type="button"
              role="tab"
              aria-selected={tab === mode}
              aria-controls={`${editorID}-${mode}-panel`}
              tabIndex={tab === mode ? 0 : -1}
              onClick={() => setTab(mode)}
              onKeyDown={(event) => {
                if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
                  event.preventDefault();
                  const next = mode === 'write' ? 'preview' : 'write';
                  setTab(next);
                  document.getElementById(`${editorID}-${next}-tab`)?.focus();
                }
              }}
            >
              {mode === 'write' ? '编辑' : '预览'}
            </button>
          ))}
        </div>
        {tab === 'write' && (
          <div className="markdown-editor__toolbar" role="group" aria-label="正文格式工具栏">
            <Dropdown
              trigger={['click']}
              menu={{
                items: [1, 2, 3, 4, 5, 6].map((level) => ({
                  key: `heading-${level}`,
                  label: `${['一', '二', '三', '四', '五', '六'][level - 1]}级标题 H${level}`,
                  title: `Ctrl/⌘ + Alt/⌥ + ${level}`,
                })),
                onClick: ({ key }) => apply(key as MarkdownAction),
              }}
            >
              <Button
                type="text"
                size="small"
                aria-label="插入标题"
                icon={<FontSizeOutlined />}
                onMouseDown={(event) => event.preventDefault()}
              >
                <DownOutlined />
              </Button>
            </Dropdown>
            {tools.map((tool) => (
              <Tooltip
                key={tool.action}
                title={`${tool.label}${'hint' in tool ? `（${tool.hint}）` : ''}`}
              >
                <Button
                  type="text"
                  size="small"
                  aria-label={tool.label}
                  icon={tool.icon}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => apply(tool.action)}
                />
              </Tooltip>
            ))}
          </div>
        )}
      </div>
      <div
        id={`${editorID}-write-panel`}
        role="tabpanel"
        aria-labelledby={`${editorID}-write-tab`}
        hidden={tab !== 'write'}
      >
        <Input.TextArea
          {...inputProps}
          id={id}
          ref={input}
          value={value}
          rows={12}
          placeholder="请输入正文"
          onChange={(event) => onChange?.(event.target.value)}
          onSelect={rememberSelection}
          onKeyDown={keyDown}
        />
      </div>
      {tab === 'preview' && (
        <div
          className="markdown-editor__preview"
          id={`${editorID}-preview-panel`}
          role="tabpanel"
          aria-labelledby={`${editorID}-preview-tab`}
          tabIndex={0}
        >
          {value.trim() ? (
            <HistoryContent content={value} />
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无正文可预览" />
          )}
        </div>
      )}
    </div>
  );
}
