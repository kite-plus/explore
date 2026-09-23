import * as React from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface Props {
  host: string | null;
  open: boolean;
  onClose: () => void;
  onConfirm: (exclude: "" | "opt_out" | "blocked", note: string) => Promise<void>;
}

export function DeleteBlogDialog({ host, open, onClose, onConfirm }: Props) {
  const [exclude, setExclude] = React.useState<"" | "opt_out" | "blocked">("");
  const [note, setNote] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (open) {
      setExclude("");
      setNote("");
      setError(null);
    }
  }, [open]);

  async function handleConfirm() {
    setLoading(true);
    setError(null);
    try {
      await onConfirm(exclude, note);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败，请重试");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={(o) => !o && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认移除 {host}？</AlertDialogTitle>
          <AlertDialogDescription>
            此操作将移除该博客及其所有抓取到的文章。
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-3">
          {/* 操作类型选择 */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">移除方式</Label>
            {[
              { value: "" as const, label: "仅移除（无排除，可重新提交）" },
              { value: "opt_out" as const, label: "移除并退出收录（写入排除名单，博主申请退出）" },
              { value: "blocked" as const, label: "移除并永久封禁（blocked，违规屏蔽）" },
            ].map((opt) => (
              <label key={opt.value} className="flex cursor-pointer items-start gap-2 text-sm">
                <input
                  type="radio"
                  name="exclude-type"
                  value={opt.value}
                  checked={exclude === opt.value}
                  onChange={() => setExclude(opt.value)}
                  className="mt-0.5 size-4"
                />
                <span className={opt.value === "blocked" ? "text-destructive" : ""}>{opt.label}</span>
              </label>
            ))}
          </div>

          {/* 备注说明（排除时显示） */}
          {exclude && (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="delete-note">说明（可选）</Label>
              <Input
                id="delete-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="补充说明，仅内部可见"
              />
            </div>
          )}

          {error && <p className="text-destructive text-sm">{error}</p>}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose} disabled={loading}>
            取消
          </AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-white hover:bg-destructive/90"
            onClick={handleConfirm}
            disabled={loading}
          >
            {loading ? "移除中…" : "确认移除"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
