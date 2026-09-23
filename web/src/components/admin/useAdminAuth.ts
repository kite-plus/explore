import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { ACCOUNT_ADMIN } from "@/lib/admin-api";
import { currentReader, readerRequest } from "@/lib/reader-api";

/** 鉴权上下文接口 */
export interface AdminAuthContext {
  /** 当前 Token（原始字符串），null 表示未登录 */
  token: string | null;
  /** 是否已登录 */
  isAuthenticated: boolean;
  /** 401 失效时的错误提示文案 */
  authError: string | null;
  /** 登录：将 token 写入 sessionStorage 或 localStorage */
  login: (token: string, remember: boolean) => void;
  /** 登出：清除 token，触发重新登录弹窗 */
  logout: () => void;
  /** Token 无效时由 adminFetch 调用（外部触发） */
  onUnauthorized: () => void;
}

const STORAGE_KEY = "explore_admin_token";

function readToken(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(STORAGE_KEY) ?? localStorage.getItem(STORAGE_KEY);
}

export const AuthContext = createContext<AdminAuthContext>({
  token: null,
  isAuthenticated: false,
  authError: null,
  login: () => {},
  logout: () => {},
  onUnauthorized: () => {},
});

export function useAdminAuth(): AdminAuthContext {
  return useContext(AuthContext);
}

/** 在 AdminShell 内部使用，创建鉴权状态 */
export function useAuthState() {
  const [token, setToken] = useState<string | null>(null);
  const [authError, setAuthError] = useState<string | null>(null);

  // 挂载后从存储中恢复 Token
  useEffect(() => {
    const stored = readToken();
    if (stored) { setToken(stored); return; }
    void fetch("/api/v1/admin/session", { credentials: "same-origin" }).then(response => {
      if (response.ok) setToken(ACCOUNT_ADMIN);
    }).catch(() => {});
  }, []);

  const login = useCallback((newToken: string, remember: boolean) => {
    if (newToken === ACCOUNT_ADMIN) {
      sessionStorage.removeItem(STORAGE_KEY);
      localStorage.removeItem(STORAGE_KEY);
      setToken(newToken);
      setAuthError(null);
      return;
    }
    sessionStorage.setItem(STORAGE_KEY, newToken);
    if (remember) {
      localStorage.setItem(STORAGE_KEY, newToken);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
    setToken(newToken);
    setAuthError(null);
  }, []);

  const logout = useCallback(() => {
    if (token === ACCOUNT_ADMIN) {
      void currentReader().then(user => user && readerRequest("auth/logout", { method: "POST" }, user.csrf_token));
    }
    sessionStorage.removeItem(STORAGE_KEY);
    localStorage.removeItem(STORAGE_KEY);
    setToken(null);
    setAuthError(null);
  }, [token]);

  const onUnauthorized = useCallback(() => {
    sessionStorage.removeItem(STORAGE_KEY);
    localStorage.removeItem(STORAGE_KEY);
    setToken(null);
    setAuthError("登录已失效，请重新登录");
  }, []);

  return { token, authError, login, logout, onUnauthorized };
}
