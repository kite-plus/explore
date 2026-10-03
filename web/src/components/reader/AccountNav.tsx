import { useEffect, useState } from "react";

import { currentReader } from "@/lib/reader-api";

type Reader = { display_name: string };

// The server draws the link from the request's session (src/middleware.ts).
// Only a page drawn without asking, which a cache may have kept from before
// the reader signed in, asks again here.
export function AccountNav({ lang, reader, checked }: { lang: "zh" | "en"; reader: Reader | null; checked: boolean }) {
  const [user, setUser] = useState<Reader | null>(reader);
  useEffect(() => {
    if (!checked) void currentReader().then(setUser);
  }, [checked]);
  const prefix = lang === "en" ? "/en" : "";
  return <a className="whitespace-nowrap text-sm text-muted-foreground hover:text-foreground" href={`${prefix}/${user ? "account" : "login"}`}>
    {user ? user.display_name : lang === "zh" ? "登录" : "Sign in"}
  </a>;
}
