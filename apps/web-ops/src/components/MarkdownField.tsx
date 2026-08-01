import { useLayoutEffect, useRef, useState, type ChangeEvent } from "react";
import ReactMarkdown from "react-markdown";
import { Button, Textarea } from "./ui";

type MarkdownFieldProps = {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  /** Minimum visible rows when empty. Grows with content. */
  rows?: number;
};

const MIN_ROWS = 8;
const MAX_HEIGHT_PX = 480;

function wrapSelection(
  el: HTMLTextAreaElement,
  before: string,
  after: string,
  emptyPlaceholder = "",
): { next: string; start: number; end: number } {
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const selected = el.value.slice(start, end);
  const insert = selected || emptyPlaceholder;
  const next = el.value.slice(0, start) + before + insert + after + el.value.slice(end);
  if (selected) {
    return { next, start: start + before.length, end: start + before.length + insert.length };
  }
  const caret = start + before.length + insert.length;
  return { next, start: caret, end: caret };
}

function bulletSelection(el: HTMLTextAreaElement): { next: string; start: number; end: number } {
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const value = el.value;
  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  const lineEnd = (() => {
    const n = value.indexOf("\n", end);
    return n === -1 ? value.length : n;
  })();
  const block = value.slice(lineStart, lineEnd);
  const lines = block.length ? block.split("\n") : [""];
  const bulleted = lines.map((line) => (line.startsWith("- ") ? line : `- ${line}`)).join("\n");
  const next = value.slice(0, lineStart) + bulleted + value.slice(lineEnd);
  return { next, start: lineStart, end: lineStart + bulleted.length };
}

function autosize(el: HTMLTextAreaElement, minRows: number) {
  const styles = window.getComputedStyle(el);
  const lineHeight = Number.parseFloat(styles.lineHeight) || 22;
  const padY =
    (Number.parseFloat(styles.paddingTop) || 0) + (Number.parseFloat(styles.paddingBottom) || 0);
  const borderY =
    (Number.parseFloat(styles.borderTopWidth) || 0) + (Number.parseFloat(styles.borderBottomWidth) || 0);
  const minHeight = Math.ceil(lineHeight * minRows + padY + borderY);

  el.style.height = "auto";
  const next = Math.min(Math.max(el.scrollHeight, minHeight), MAX_HEIGHT_PX);
  el.style.height = `${next}px`;
  el.style.overflowY = el.scrollHeight > MAX_HEIGHT_PX ? "auto" : "hidden";
}

export function MarkdownField({
  label,
  value,
  onChange,
  placeholder,
  rows = MIN_ROWS,
}: MarkdownFieldProps) {
  const [preview, setPreview] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const minRows = Math.max(rows, MIN_ROWS);

  useLayoutEffect(() => {
    if (preview) return;
    const el = textareaRef.current;
    if (!el) return;
    autosize(el, minRows);
  }, [value, preview, minRows]);

  function applyEdit(edit: { next: string; start: number; end: number }) {
    onChange(edit.next);
    requestAnimationFrame(() => {
      const el = textareaRef.current;
      if (!el) return;
      el.focus();
      el.setSelectionRange(edit.start, edit.end);
      autosize(el, minRows);
    });
  }

  function onBold() {
    const el = textareaRef.current;
    if (!el) return;
    applyEdit(wrapSelection(el, "**", "**", "bold text"));
  }

  function onItalic() {
    const el = textareaRef.current;
    if (!el) return;
    applyEdit(wrapSelection(el, "_", "_", "italic text"));
  }

  function onBullet() {
    const el = textareaRef.current;
    if (!el) return;
    applyEdit(bulletSelection(el));
  }

  function onTextChange(e: ChangeEvent<HTMLTextAreaElement>) {
    onChange(e.target.value);
    autosize(e.target, minRows);
  }

  return (
    <div className="markdown-field">
      <div className="markdown-field-head">
        <span className="markdown-field-label">{label}</span>
        <div className="btn-row" style={{ gap: 4, margin: 0 }}>
          {!preview ? (
            <>
              <Button type="button" variant="ghost" className="btn-sm" onClick={onBold} title="Bold">
                Bold
              </Button>
              <Button type="button" variant="ghost" className="btn-sm" onClick={onItalic} title="Italic">
                Italic
              </Button>
              <Button type="button" variant="ghost" className="btn-sm" onClick={onBullet} title="Bullet list">
                List
              </Button>
            </>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            className="btn-sm"
            onClick={() => setPreview((v) => !v)}
            aria-pressed={preview}
          >
            {preview ? "Edit" : "Preview"}
          </Button>
        </div>
      </div>
      {preview ? (
        <div className="markdown-field-preview input">
          {value.trim() ? <ReactMarkdown>{value}</ReactMarkdown> : <span className="muted">Nothing to preview</span>}
        </div>
      ) : (
        <Textarea
          ref={textareaRef}
          className="markdown-field-textarea"
          value={value}
          rows={minRows}
          placeholder={placeholder}
          onChange={onTextChange}
        />
      )}
      <span className="hint">Supports **bold**, _italic_, and lists.</span>
    </div>
  );
}
