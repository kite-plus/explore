import { useEffect, useState, type ComponentProps } from "react";
import { ArrowLeft, ArrowRight, CircleAlert, Eye, EyeOff } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button, ButtonLink } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Spinner } from "@/components/ui/spinner";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

export function AuthForm({ lang, next }: { lang: "zh" | "en"; next: string }) {
  const [register, setRegister] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const zh = lang === "zh";
  const home = zh ? "/" : "/en/";
  const title = register ? (zh ? "创建账号" : "Create an account") : (zh ? "登录 Explore" : "Sign in to Explore");

  useEffect(() => {
    const controller = new AbortController();

    void fetch("/api/v1/site-config", { signal: controller.signal })
      .then(response => response.ok ? response.json() : null)
      .then(result => {
        if (result?.registration_enabled === false) {
          setRegistrationEnabled(false);
          setRegister(false);
        }
      })
      .catch(() => {});

    return () => controller.abort();
  }, []);

  const submit: NonNullable<ComponentProps<"form">["onSubmit"]> = async event => {
    event.preventDefault();
    if (busy) return;

    setBusy(true);
    setError("");
    try {
      await readerRequest<ReaderUser>("auth/" + (register ? "register" : "login"), {
        method: "POST",
        body: JSON.stringify({ email, password, ...(register ? { display_name: name } : {}) }),
      });
      const fallback = zh ? "/following" : "/en/following";
      window.location.assign(next.startsWith("/") && !next.startsWith("//") ? next : fallback);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : zh ? "暂时无法登录，请重试。" : "Unable to sign in. Please try again.");
    } finally {
      setBusy(false);
    }
  };

  function switchMode() {
    setRegister(value => !value);
    setShowPassword(false);
    setError("");
  }

  return (
    <section aria-labelledby="auth-title" className="mx-auto my-3 w-full max-w-[26rem] sm:my-7">
      <Card className="auth-panel rounded-2xl [--card-spacing:--spacing(6)] sm:[--card-spacing:--spacing(8)]">
        <CardHeader className="justify-items-center gap-2.5 text-center">
          <div className="mb-3 flex size-12 items-center justify-center rounded-xl border bg-background shadow-xs" aria-hidden="true">
            <img src="/favicon.svg" alt="" width={30} height={25} />
          </div>
          <CardTitle>
            <h1 id="auth-title" className="text-[1.625rem] font-semibold tracking-tight">{title}</h1>
          </CardTitle>
          <CardDescription>
            <p className="text-pretty leading-relaxed">
              {register
                ? zh ? "订阅喜欢的博客，拥有自己的阅读流。" : "Follow your favorite blogs in your own reading feed."
                : zh ? "欢迎回来，接着读你喜欢的博客。" : "Welcome back. Pick up where you left off."}
            </p>
          </CardDescription>
        </CardHeader>

        <CardContent>
          <form onSubmit={submit} aria-labelledby="auth-title" aria-busy={busy}>
            <FieldGroup>
              {register && (
                <Field data-disabled={busy}>
                  <FieldLabel htmlFor="auth-name">{zh ? "显示名称" : "Display name"}</FieldLabel>
                  <Input
                    id="auth-name"
                    name="display_name"
                    value={name}
                    onChange={event => setName(event.target.value)}
                    required
                    disabled={busy}
                    maxLength={80}
                    autoComplete="name"
                    placeholder={zh ? "你的名字或昵称" : "Your name or nickname"}
                    variant="soft"
                    className="h-11 px-3.5"
                  />
                </Field>
              )}

              <Field data-disabled={busy}>
                <FieldLabel htmlFor="auth-email">{zh ? "邮箱地址" : "Email address"}</FieldLabel>
                <Input
                  id="auth-email"
                  name="email"
                  type="email"
                  value={email}
                  onChange={event => setEmail(event.target.value)}
                  required
                  disabled={busy}
                  autoComplete="email"
                  autoCapitalize="none"
                  spellCheck={false}
                  placeholder="you@example.com"
                  variant="soft"
                  className="h-11 px-3.5"
                />
              </Field>

              <Field data-disabled={busy}>
                <FieldLabel htmlFor="auth-password">{zh ? "密码" : "Password"}</FieldLabel>
                <InputGroup variant="soft" className="h-11">
                  <InputGroupInput
                    id="auth-password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    value={password}
                    onChange={event => setPassword(event.target.value)}
                    required
                    disabled={busy}
                    minLength={register ? 12 : undefined}
                    autoComplete={register ? "new-password" : "current-password"}
                    aria-describedby={register ? "auth-password-hint" : undefined}
                    placeholder={register ? zh ? "至少 12 个字符" : "At least 12 characters" : zh ? "输入你的密码" : "Enter your password"}
                    className="h-full pl-3.5"
                  />
                  <InputGroupAddon align="inline-end" className="pr-2.5">
                    <InputGroupButton
                      size="icon-sm"
                      disabled={busy}
                      aria-controls="auth-password"
                      aria-pressed={showPassword}
                      aria-label={showPassword ? zh ? "隐藏密码" : "Hide password" : zh ? "显示密码" : "Show password"}
                      title={showPassword ? zh ? "隐藏密码" : "Hide password" : zh ? "显示密码" : "Show password"}
                      onClick={() => setShowPassword(value => !value)}
                    >
                      {showPassword ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                    </InputGroupButton>
                  </InputGroupAddon>
                </InputGroup>
                {register && (
                  <FieldDescription id="auth-password-hint">
                    {zh ? "密码至少需要 12 个字符。" : "Use at least 12 characters for your password."}
                  </FieldDescription>
                )}
              </Field>

              {error && (
                <Alert variant="destructive">
                  <CircleAlert aria-hidden="true" />
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <Button
                type="submit"
                disabled={busy}
                size="lg"
                aria-live="polite"
                className="group mt-1 h-11 w-full rounded-lg shadow-sm transition-[background-color,box-shadow,transform] hover:shadow-md active:translate-y-px motion-reduce:transform-none motion-reduce:transition-none"
              >
                {busy
                  ? zh ? "请稍候…" : "Please wait…"
                  : register ? zh ? "创建账号" : "Create account" : zh ? "登录" : "Sign in"}
                {busy
                  ? <Spinner data-icon="inline-end" aria-hidden="true" className="motion-reduce:animate-none" />
                  : <ArrowRight data-icon="inline-end" aria-hidden="true" className="transition-transform group-hover:translate-x-0.5 motion-reduce:transform-none motion-reduce:transition-none" />}
              </Button>
            </FieldGroup>
          </form>
        </CardContent>

        <CardFooter className="flex-wrap justify-center gap-x-1 py-4">
          {registrationEnabled ? (
            <>
              <span className="text-muted-foreground">
                {register ? zh ? "已有账号？" : "Already have an account?" : zh ? "还没有账号？" : "New to Explore?"}
              </span>
              <Button type="button" variant="link" disabled={busy} onClick={switchMode} className="h-auto px-1 py-1">
                {register ? zh ? "返回登录" : "Sign in" : zh ? "创建账号" : "Create an account"}
              </Button>
            </>
          ) : (
            <p className="text-muted-foreground">{zh ? "目前暂未开放注册。" : "Registration is currently closed."}</p>
          )}
        </CardFooter>
      </Card>

      <div className="mt-5 flex justify-center text-muted-foreground">
        <ButtonLink href={home} variant="ghost" size="sm" className="gap-2">
          <ArrowLeft data-icon="inline-start" aria-hidden="true" />
          {zh ? "暂不登录，先逛逛" : "Browse without signing in"}
        </ButtonLink>
      </div>
    </section>
  );
}
