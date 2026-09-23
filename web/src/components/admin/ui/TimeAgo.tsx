import * as React from "react";

/** 将 ISO 时间字符串渲染为中文相对时间，tooltip 显示绝对时间 */
export function TimeAgo({ iso, className }: { iso: string | null | undefined; className?: string }) {
  const [label, setLabel] = React.useState<string>("—");

  React.useEffect(() => {
    if (!iso) {
      setLabel("—");
      return;
    }
    setLabel(formatRelative(iso));
    // 每分钟更新一次相对时间
    const timer = setInterval(() => setLabel(formatRelative(iso)), 60_000);
    return () => clearInterval(timer);
  }, [iso]);

  if (!iso) return <span className={className ?? "text-muted-foreground"}>—</span>;

  const absolute = new Date(iso).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <time
      dateTime={iso}
      title={absolute}
      className={className ?? "text-muted-foreground cursor-default"}
    >
      {label}
    </time>
  );
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const abs = Math.abs(diff);
  const future = diff < 0;

  let text: string;
  if (abs < 60_000) {
    text = "刚刚";
  } else if (abs < 3_600_000) {
    text = `${Math.floor(abs / 60_000)} 分钟${future ? "后" : "前"}`;
  } else if (abs < 86_400_000) {
    text = `${Math.floor(abs / 3_600_000)} 小时${future ? "后" : "前"}`;
  } else if (abs < 2_592_000_000) {
    text = `${Math.floor(abs / 86_400_000)} 天${future ? "后" : "前"}`;
  } else {
    text = new Date(iso).toLocaleDateString("zh-CN");
  }
  return text;
}
