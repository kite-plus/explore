import * as React from "react";
import { XIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface Props {
  /** 当前已选中的标签列表 */
  value: string[];
  /** 变更回调 */
  onChange: (tags: string[]) => void;
  /** 允许的最大标签数 */
  max?: number;
  placeholder?: string;
  className?: string;
}

/**
 * 标签输入组件
 * - 按 Enter / 空格 / 逗号确认添加
 * - 点击 × 删除
 * - 超过 max 时禁用输入
 */
export function TagInput({ value, onChange, max = 20, placeholder = "输入后按 Enter 添加", className }: Props) {
  const [input, setInput] = React.useState("");
  const inputRef = React.useRef<HTMLInputElement>(null);

  function addTag(raw: string) {
    const tag = raw.trim().toLowerCase();
    if (!tag || value.includes(tag)) return;
    if (max && value.length >= max) return;
    onChange([...value, tag]);
    setInput("");
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === " " || e.key === ",") {
      e.preventDefault();
      addTag(input);
    } else if (e.key === "Backspace" && !input && value.length > 0) {
      // 退格键删除最后一个标签
      onChange(value.slice(0, -1));
    }
  }

  function removeTag(tag: string) {
    onChange(value.filter((t) => t !== tag));
  }

  const disabled = max !== undefined && value.length >= max;

  return (
    <div
      className={cn(
        "flex flex-wrap gap-1.5 rounded-md border border-input bg-background p-2 cursor-text",
        className,
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((tag) => (
        <span
          key={tag}
          className="inline-flex items-center gap-1 rounded bg-accent px-2 py-0.5 text-xs text-accent-foreground"
        >
          {tag}
          <button
            type="button"
            onClick={() => removeTag(tag)}
            className="text-muted-foreground hover:text-foreground"
            aria-label={`删除 ${tag}`}
          >
            <XIcon className="size-3" />
          </button>
        </span>
      ))}
      {!disabled && (
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={() => addTag(input)}
          placeholder={value.length === 0 ? placeholder : ""}
          className="min-w-24 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
      )}
    </div>
  );
}
