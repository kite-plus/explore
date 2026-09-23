import { useEffect, useState } from "react";

import { currentReader, type ReaderUser } from "@/lib/reader-api";

export function AccountNav({ lang }: { lang: "zh" | "en" }) {
  const [user, setUser] = useState<ReaderUser | null>(null);
  useEffect(() => { void currentReader().then(setUser); }, []);
  const prefix = lang === "en" ? "/en" : "";
  return <a className="whitespace-nowrap text-sm text-muted-foreground hover:text-foreground" href={`${prefix}/${user ? "account" : "login"}`}>
    {user ? user.display_name : lang === "zh" ? "登录" : "Sign in"}
  </a>;
}
