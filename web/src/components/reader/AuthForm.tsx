import { useEffect, useState, type ComponentProps, type ReactNode } from "react";
import { CircleAlert, Eye, EyeOff, LogIn, UserPlus } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const MIN_PASSWORD = 12;

type Errors = Partial<Record<"name" | "email" | "password", string>>;

// Laid out like the admin sign-in (src/admin/features/auth/sign-in.tsx), so
// both read as one site.
export function AuthForm({ lang, next }: { lang: "zh" | "en"; next: string }) {
  const [register, setRegister] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const zh = lang === "zh";
  const home = zh ? "/" : "/en/";
  // As in the admin: checked on submit, then on every change.
  const errors = submitted ? validate() : {};

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

  function validate(): Errors {
    const found: Errors = {};
    if (register && !name.trim()) found.name = zh ? "请输入名称。" : "Enter a name.";
    if (!email.trim()) found.email = zh ? "请输入邮箱。" : "Enter your email.";
    else if (!EMAIL.test(email.trim())) found.email = zh ? "邮箱格式不正确。" : "Enter a valid email.";
    if (!password) found.password = zh ? "请输入密码。" : "Enter your password.";
    else if (register && password.length < MIN_PASSWORD) {
      found.password = zh ? `密码至少 ${MIN_PASSWORD} 位。` : `Use at least ${MIN_PASSWORD} characters.`;
    }
    return found;
  }

  const submit: NonNullable<ComponentProps<"form">["onSubmit"]> = async event => {
    event.preventDefault();
    setSubmitted(true);
    if (busy || Object.keys(validate()).length > 0) return;

    setBusy(true);
    setError("");
    try {
      await readerRequest<ReaderUser>("auth/" + (register ? "register" : "login"), {
        method: "POST",
        headers: { "Accept-Language": zh ? "zh-CN" : "en" },
        body: JSON.stringify({ email: email.trim(), password, ...(register ? { display_name: name.trim() } : {}) }),
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
    setSubmitted(false);
    setError("");
  }

  return (
    <section aria-labelledby="auth-title" className="mx-auto flex w-full max-w-sm flex-col gap-6 sm:pb-16">
      <div className="flex flex-col gap-2">
        <h1 id="auth-title" className="text-lg font-semibold tracking-tight">
          {register ? zh ? "创建账号" : "Create an account" : zh ? "登录" : "Sign in"}
        </h1>
        <p className="text-sm text-muted-foreground">
          {register
            ? zh ? "订阅喜欢的博客，拥有自己的阅读流。" : "Follow your favorite blogs in your own reading feed. "
            : zh ? "输入你的邮箱和密码。" : "Enter your email and password. "}
          <br className="max-sm:hidden" />
          {registrationEnabled ? (
            <>
              {register ? zh ? "已有账号？" : "Already have an account?" : zh ? "还没有账号？" : "New to Explore?"}{" "}
              <button
                type="button"
                disabled={busy}
                onClick={switchMode}
                className="cursor-pointer text-nowrap underline underline-offset-4 hover:text-primary disabled:cursor-not-allowed"
              >
                {register ? zh ? "返回登录" : "Sign in" : zh ? "创建账号" : "Create an account"}
              </button>
            </>
          ) : zh ? "目前暂未开放注册。" : "Registration is currently closed."}
        </p>
      </div>

      <form onSubmit={submit} noValidate aria-labelledby="auth-title" aria-busy={busy}>
        <FieldGroup className="gap-3">
          {register && (
            <Item id="auth-name" label={zh ? "名称" : "Name"} error={errors.name} busy={busy}>
              <Input
                id="auth-name"
                name="display_name"
                value={name}
                onChange={event => setName(event.target.value)}
                disabled={busy}
                maxLength={80}
                autoComplete="name"
                placeholder={zh ? "你的名字或昵称" : "Your name or nickname"}
                aria-invalid={Boolean(errors.name)}
                aria-describedby={errors.name ? "auth-name-message" : undefined}
              />
            </Item>
          )}

          <Item id="auth-email" label={zh ? "邮箱" : "Email"} error={errors.email} busy={busy}>
            <Input
              id="auth-email"
              name="email"
              type="email"
              value={email}
              onChange={event => setEmail(event.target.value)}
              disabled={busy}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="name@example.com"
              aria-invalid={Boolean(errors.email)}
              aria-describedby={errors.email ? "auth-email-message" : undefined}
            />
          </Item>

          <Item
            id="auth-password"
            label={zh ? "密码" : "Password"}
            error={errors.password}
            hint={register ? zh ? `至少 ${MIN_PASSWORD} 位。` : `At least ${MIN_PASSWORD} characters.` : undefined}
            busy={busy}
          >
            <PasswordInput
              key={register ? "new" : "current"}
              zh={zh}
              id="auth-password"
              name="password"
              value={password}
              onChange={event => setPassword(event.target.value)}
              disabled={busy}
              autoComplete={register ? "new-password" : "current-password"}
              placeholder={register ? undefined : "********"}
              aria-invalid={Boolean(errors.password)}
              aria-describedby={errors.password || register ? "auth-password-message" : undefined}
            />
          </Item>

          {error && (
            <Alert variant="destructive">
              <CircleAlert aria-hidden="true" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <Button type="submit" disabled={busy} className="mt-2">
            {busy ? <Spinner aria-hidden="true" /> : register ? <UserPlus aria-hidden="true" /> : <LogIn aria-hidden="true" />}
            {register ? zh ? "创建账号" : "Create account" : zh ? "登录" : "Sign in"}
          </Button>
        </FieldGroup>
      </form>

      <p className="px-8 text-center text-sm text-muted-foreground">
        {zh ? "不登录也能阅读，" : "Reading needs no account. "}
        <a href={home} className="text-nowrap underline underline-offset-4 hover:text-primary">
          {zh ? "返回首页" : "Start reading"}
        </a>
        {zh ? "。" : "."}
      </p>
    </section>
  );
}

function Item({ id, label, error, hint, busy, children }: {
  id: string;
  label: string;
  error?: string;
  hint?: string;
  busy: boolean;
  children: ReactNode;
}) {
  return (
    <Field data-invalid={Boolean(error)} data-disabled={busy}>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {children}
      {error
        ? <FieldError id={`${id}-message`}>{error}</FieldError>
        : hint && <FieldDescription id={`${id}-message`}>{hint}</FieldDescription>}
    </Field>
  );
}

// The admin's PasswordInput, built from the reader's own components.
function PasswordInput({ zh, disabled, ...props }: ComponentProps<typeof Input> & { zh: boolean }) {
  const [shown, setShown] = useState(false);
  // A fixed name, since aria-pressed already says whether it is shown.
  const label = zh ? "显示密码" : "Show password";
  return (
    <div className="relative">
      <Input {...props} type={shown ? "text" : "password"} disabled={disabled} className="pe-9" />
      <Button
        type="button"
        variant="ghost"
        disabled={disabled}
        aria-controls={props.id}
        aria-pressed={shown}
        aria-label={label}
        title={label}
        onClick={() => setShown(value => !value)}
        className="absolute end-1 top-1/2 size-6 -translate-y-1/2 cursor-pointer p-0 text-muted-foreground"
      >
        {shown ? <Eye className="size-4.5" aria-hidden="true" /> : <EyeOff className="size-4.5" aria-hidden="true" />}
      </Button>
    </div>
  );
}
