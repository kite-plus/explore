import { useEffect, useState } from "react";
import { ChevronDown, LogOut, Rss, ShieldCheck, UserRound } from "lucide-react";

import { avatarColor, initial } from "@/lib/avatar";
import { currentReader, readerRequest, type Reader } from "@/lib/reader-api";
import { cn } from "@/lib/utils";

// The account corner of the header. The server draws it from the request's
// session (src/middleware.ts); only a page drawn without asking, which a
// cache may have kept from before the reader signed in, asks again here.
export function AccountMenu({ lang, reader, checked }: { lang: "zh" | "en"; reader: Reader | null; checked: boolean }) {
  const [user, setUser] = useState<Reader | null>(reader);
  const [leaving, setLeaving] = useState(false);
  const zh = lang === "zh";
  const prefix = zh ? "" : "/en";

  useEffect(() => {
    if (!checked) void currentReader().then(setUser);
  }, [checked]);

  if (!user) {
    return (
      <a href={`${prefix}/login`} className="whitespace-nowrap rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-accent hover:text-accent-foreground">
        {zh ? "登录" : "Sign in"}
      </a>
    );
  }

  async function signOut() {
    setLeaving(true);
    try {
      const me = await currentReader();
      if (me) await readerRequest("auth/logout", { method: "POST" }, me.csrf_token);
    } finally {
      window.location.assign(`${prefix}/`);
    }
  }

  const item = "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-sm text-foreground outline-offset-1 hover:bg-accent hover:text-accent-foreground [&>svg]:size-4 [&>svg]:text-muted-foreground";
  return (
    <details data-menu="" className="group relative">
      <summary
        aria-label={zh ? `${user.display_name}，账号菜单` : `${user.display_name}, account menu`}
        className="flex cursor-pointer list-none items-center gap-2 rounded-full p-0.5 outline-offset-2 hover:bg-accent sm:pe-2 [&::-webkit-details-marker]:hidden"
      >
        <Avatar user={user} />
        <span className="max-w-32 truncate text-sm font-medium max-sm:hidden">{user.display_name}</span>
        <ChevronDown aria-hidden="true" className="size-3.5 text-muted-foreground transition-transform group-open:rotate-180 max-sm:hidden" />
      </summary>
      <div className="absolute right-0 top-full z-30 mt-2 w-60 rounded-lg border bg-background p-1.5 shadow-lg">
        <div className="flex items-center gap-3 px-2.5 py-2">
          <Avatar user={user} large />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{user.display_name}</p>
            <p className="truncate text-xs text-muted-foreground">{user.email}</p>
          </div>
        </div>
        <div className="-mx-1.5 my-1.5 h-px bg-border" />
        <a className={item} href={`${prefix}/following`}>
          <Rss aria-hidden="true" />
          {zh ? "订阅流" : "Following"}
        </a>
        <a className={item} href={`${prefix}/account`}>
          <UserRound aria-hidden="true" />
          {zh ? "我的账号" : "My account"}
        </a>
        {user.is_admin && (
          <a className={item} href="/admin">
            <ShieldCheck aria-hidden="true" />
            {zh ? "管理后台" : "Admin"}
          </a>
        )}
        <div className="-mx-1.5 my-1.5 h-px bg-border" />
        <button type="button" className={cn(item, "cursor-pointer disabled:cursor-wait")} disabled={leaving} onClick={() => void signOut()}>
          <LogOut aria-hidden="true" />
          {leaving ? zh ? "正在退出…" : "Signing out…" : zh ? "退出登录" : "Sign out"}
        </button>
      </div>
    </details>
  );
}

// The first letter of the name, on a color the account's id picks.
function Avatar({ user, large = false }: { user: Reader; large?: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex shrink-0 select-none items-center justify-center rounded-full font-medium text-white",
        avatarColor(user.id),
        large ? "size-9 text-base" : "size-7 text-xs",
      )}
    >
      {initial(user.display_name)}
    </span>
  );
}
