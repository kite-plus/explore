import { useEffect, useState, type ComponentProps, type ReactNode } from "react";
import { CircleAlert, Eye, EyeOff, KeyRound, LogIn, UserPlus } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button, ButtonLink } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { safeNext } from "@/lib/next";
import { readerRequest, type ReaderUser } from "@/lib/reader-api";

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const MIN_PASSWORD = 12;

type Errors = Partial<Record<"name" | "email" | "password" | "confirm", string>>;

// Why a sign-in with Kite Plus came back without signing in; the API names
// them in ?error= (internal/api/oidc.go).
const kitePlusErrors: Record<string, [string, string]> = {
  login_expired: ["登录已过期，请重试。", "The sign-in expired. Please try again."],
  login_failed: ["没能用 Kite Plus 账号登录，请重试。", "Signing in with Kite Plus did not work. Please try again."],
  access_denied: ["Kite Plus 账号登录已取消。", "Signing in with Kite Plus was cancelled."],
  unavailable: ["Kite Plus 账号暂时无法访问，请稍后再试。", "Kite Plus accounts are unavailable right now. Please try again later."],
  account_disabled: ["这个账号已停用。", "This account is disabled."],
  email_in_use: ["本站另一个账号已在使用这个邮箱，无法自动关联到它。请联系站点。", "Another account here already uses this email and cannot be linked to it automatically. Contact the site."],
  email_unverified: ["请先在 Kite Plus 账号里验证邮箱。", "Verify your email in your Kite Plus account first."],
  registration_closed: ["目前暂未开放注册。", "Registration is currently closed."],
};

interface Props {
  lang: "zh" | "en";
  next: string;
  /** The name of the Kite Plus sign-in, or null when the server has none. */
  kitePlus?: string | null;
  /** Whether the server takes passwords; with Kite Plus it never does. */
  passwords?: boolean;
  error?: string | null;
}

// The sign-in page. With Kite Plus on the server there are no passwords and
// the page only shows when a sign-in came back with an error, to say so and
// try again (docs/design/accounts.md section 1); otherwise it is the email
// and password form.
export function AuthForm(props: Props) {
  return props.passwords === false ? <KitePlusSignIn {...props} /> : <PasswordForm {...props} />;
}

function KitePlusSignIn({ lang, next, kitePlus = null, error: returned = null }: Props) {
  const zh = lang === "zh";
  const target = safeNext(next, zh ? "/following" : "/en/following");
  const error = kitePlus === null
    ? ["暂时无法登录，请稍后再试。", "Signing in is unavailable right now. Please try again later."]
    : returned ? kitePlusErrors[returned] ?? kitePlusErrors.login_failed : null;
  const retry = kitePlus === null
    ? `${zh ? "" : "/en"}/login?next=${encodeURIComponent(target)}`
    : `/auth/login?return=${encodeURIComponent(target)}`;
  return (
    <section aria-labelledby="auth-title" className="mx-auto flex w-full max-w-sm flex-col gap-6 sm:pb-16">
      <div className="flex flex-col gap-2">
        <h1 id="auth-title" className="text-lg font-semibold tracking-tight">{zh ? "登录" : "Sign in"}</h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          {zh ? "Explore 用 Kite Plus 账号登录。" : "Explore signs you in with a Kite Plus account."}
        </p>
      </div>
      {error && (
        <Alert variant="destructive">
          <CircleAlert aria-hidden="true" />
          <AlertDescription>{zh ? error[0] : error[1]}</AlertDescription>
        </Alert>
      )}
      <ButtonLink href={retry} data-kite-plus>
        <KeyRound aria-hidden="true" />
        {kitePlus === null ? (zh ? "重试" : "Try again") : (zh ? "使用 Kite Plus 账号登录" : "Sign in with a Kite Plus account")}
      </ButtonLink>
      <div className="flex flex-col gap-3 px-8 text-center text-sm leading-relaxed text-muted-foreground">
        <p>
          {zh
            ? "以前在本站用邮箱注册过？用同一个邮箱的 Kite Plus 账号登录，就会回到原来的账号，订阅都还在。"
            : "Made an account here with an email before? Sign in with a Kite Plus account of the same email to get back to it, follows and all."}
        </p>
        <p>
          {zh ? "不登录也能阅读，" : "Reading needs no account. "}
          <a href={zh ? "/" : "/en/"} className="text-nowrap underline underline-offset-4 hover:text-primary">{zh ? "返回首页" : "Start reading"}</a>
          {zh ? "。" : "."}
        </p>
      </div>
    </section>
  );
}

// Laid out like the admin sign-in (src/admin/features/auth/sign-in.tsx), so
// both read as one site.
function PasswordForm({ lang, next }: Props) {
  const [register, setRegister] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
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
    } else if (register && confirm !== password) {
      found.confirm = zh ? "两次输入的密码不一致。" : "The passwords do not match.";
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
      const account = await readerRequest<ReaderUser>("auth/" + (register ? "register" : "login"), {
        method: "POST",
        headers: { "Accept-Language": zh ? "zh-CN" : "en" },
        body: JSON.stringify({ email: email.trim(), password, ...(register ? { display_name: name.trim() } : {}) }),
      });
      // A password an admin reset is changed first, before going on.
      window.location.assign(account.temporary_password
        ? (zh ? "/account#password" : "/en/account#password")
        : safeNext(next, zh ? "/following" : "/en/following"));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : zh ? "暂时无法登录，请重试。" : "Unable to sign in. Please try again.");
    } finally {
      setBusy(false);
    }
  };

  function switchMode() {
    setRegister(value => !value);
    setConfirm("");
    setSubmitted(false);
    setError("");
  }

  return (
    <section aria-labelledby="auth-title" className="mx-auto flex w-full max-w-sm flex-col gap-6 sm:pb-16">
      <div className="flex flex-col gap-2">
        <h1 id="auth-title" className="text-lg font-semibold tracking-tight">
          {register ? zh ? "创建账号" : "Create an account" : zh ? "登录" : "Sign in"}
        </h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          {register
            ? zh ? "订阅喜欢的博客，拥有自己的阅读流。" : "Follow your favorite blogs in your own reading feed."
            : zh ? "输入你的邮箱和密码。" : "Enter your email and password."}
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

          {register && (
            <Item id="auth-confirm" label={zh ? "确认密码" : "Confirm password"} error={errors.confirm} busy={busy}>
              <PasswordInput
                zh={zh}
                id="auth-confirm"
                name="confirm"
                value={confirm}
                onChange={event => setConfirm(event.target.value)}
                disabled={busy}
                autoComplete="new-password"
                aria-invalid={Boolean(errors.confirm)}
                aria-describedby={errors.confirm ? "auth-confirm-message" : undefined}
              />
            </Item>
          )}

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

      <div className="flex flex-col gap-3 px-8 text-center text-sm leading-relaxed text-muted-foreground">
        <p>
          {registrationEnabled ? (
            <>
              {register ? zh ? "已有账号？" : "Already have an account?" : zh ? "还没有账号？" : "New to Explore?"}{" "}
              <button
                type="button"
                disabled={busy}
                onClick={switchMode}
                className="cursor-pointer text-nowrap font-medium text-foreground underline underline-offset-4 hover:text-primary disabled:cursor-not-allowed"
              >
                {register ? zh ? "返回登录" : "Sign in" : zh ? "创建账号" : "Create an account"}
              </button>
            </>
          ) : zh ? "目前暂未开放注册。" : "Registration is currently closed."}
        </p>
        <p>
          {zh ? "不登录也能阅读，" : "Reading needs no account. "}
          <a href={home} className="text-nowrap underline underline-offset-4 hover:text-primary">
            {zh ? "返回首页" : "Start reading"}
          </a>
          {zh ? "。" : "."}
        </p>
      </div>
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
