import type {
  ButtonHTMLAttributes,
  ChangeEvent,
  InputHTMLAttributes,
  ReactNode,
  TextareaHTMLAttributes,
} from "react";
import { forwardRef, useRef, useState } from "react";

function combineClasses(base: string, extra?: string) {
  return extra ? `${base} ${extra}`.trim() : base;
}

export function Button({
  variant = "primary",
  className,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "gold" | "danger";
}) {
  return (
    <button className={combineClasses(`btn btn-${variant}`, className)} {...props}>
      {children}
    </button>
  );
}

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className, ...props },
  ref,
) {
  return <input ref={ref} className={combineClasses("input", className)} {...props} />;
});

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={combineClasses("input", className)} {...props} />;
}

/** Text input with a leading search icon — use for free-text filters. */
export const SearchInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function SearchInput({ className, ...props }, ref) {
    return (
      <span className="search-input">
        <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} size={16} />
        <input ref={ref} className={combineClasses("input", className)} {...props} />
      </span>
    );
  },
);

/** Password field with a show/hide toggle. */
export function PasswordInput({
  className,
  ...props
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type">) {
  const [visible, setVisible] = useState(false);
  return (
    <span className="password-input">
      <input
        className={combineClasses("input", className)}
        {...props}
        type={visible ? "text" : "password"}
      />
      <button
        type="button"
        className="password-input-toggle"
        onClick={() => setVisible((v) => !v)}
        aria-label={visible ? "Hide password" : "Show password"}
        aria-pressed={visible}
        tabIndex={-1}
      >
        {visible ? (
          <Icon
            d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"
            extra={
              <>
                <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                <path d="M14.12 14.12a3 3 0 1 1-4.24-4.24" />
                <path d="m1 1 22 22" />
              </>
            }
            size={16}
          />
        ) : (
          <Icon
            d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"
            extra={<circle cx="12" cy="12" r="3" />}
            size={16}
          />
        )}
      </button>
    </span>
  );
}

export function Badge({
  tone = "info",
  children,
}: {
  tone?: "success" | "warning" | "danger" | "info" | "pending";
  children: ReactNode;
}) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

/** Generic inline SVG icon — pass a primary path plus any extra path/circle elements. */
export function Icon({
  d,
  extra,
  size = 18,
}: {
  d: string;
  extra?: ReactNode;
  size?: number;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={d} />
      {extra}
    </svg>
  );
}

/** Shared icon set used across pages for consistent iconography. */
export const ICONS: Record<string, ReactNode> = {
  repairs: <Icon d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />,
  customers: <Icon d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" extra={<><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>} />,
  parts: <Icon d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" extra={<path d="m3.29 7 8.71 5 8.71-5M12 22V12" />} />,
  inventory: <Icon d="M22 8.35V20a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V8.35A2 2 0 0 1 3.26 6.5l8-3.2a2 2 0 0 1 1.48 0l8 3.2A2 2 0 0 1 22 8.35z" />,
  suppliers: <Icon d="M5 8h14M5 8a2 2 0 1 0 0-4h14a2 2 0 1 0 0 4M5 8v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8" />,
  pos: <Icon d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" extra={<path d="M3 6h18m-5 4a4 4 0 0 1-8 0" />} />,
  orders: <Icon d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" extra={<path d="M3 6h18m-5 4a4 4 0 0 1-8 0" />} />,
  cash: <Icon d="M2 7h20v10H2z" extra={<circle cx="12" cy="12" r="2.5" />} />,
  risk: <Icon d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3z" extra={<path d="M12 9v4m0 4h.01" />} />,
  reports: <Icon d="M3 3v18h18" extra={<path d="m19 9-5 5-4-4-3 3" />} />,
  audit: <Icon d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" extra={<path d="M14 2v6h6M9 13h6M9 17h6" />} />,
  sync: <Icon d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" extra={<path d="M3 3v5h5m-5 4a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16m0 5v-5h-5" />} />,
  settings: <Icon d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" extra={<circle cx="12" cy="12" r="3" />} />,
  shortage: <Icon d="M12 2v20m5-15H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />,
  stk: <Icon d="M5 12.5a7 7 0 0 1 14 0M8.5 13.5a3.5 3.5 0 0 1 7 0" extra={<circle cx="12" cy="13.5" r="1" />} />,
  ready: <Icon d="M20 6 9 17l-5-5" />,
  clock: <Icon d="M12 6v6l4 2" extra={<circle cx="12" cy="12" r="9" />} />,
  hash: <Icon d="M4 9h16M4 15h16M10 3 8 21M16 3l-2 18" />,
  package: <Icon d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" extra={<path d="m3.29 7 8.71 5 8.71-5M12 22V12" />} />,
  boxes: <Icon d="M2 7h20v10H2z" extra={<circle cx="12" cy="12" r="2.5" />} />,
  search: <Icon d="m21 21-4.34-4.34" extra={<circle cx="11" cy="11" r="7" />} />,
  inbox: <Icon d="M22 12h-6l-2 3h-4l-2-3H2" extra={<path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />} />,
};

export function Avatar({ name, size = 32 }: { name: string; size?: number }) {
  const initials =
    name
      .trim()
      .split(/\s+/)
      .map((p) => p[0])
      .slice(0, 2)
      .join("")
      .toUpperCase() || "?";
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0;
  const hue = hash % 360;
  return (
    <span
      className="avatar"
      style={{
        width: size,
        height: size,
        fontSize: size * 0.36,
        background: `color-mix(in srgb, hsl(${hue} 55% 45%) 16%, var(--color-background-subtle))`,
        color: `hsl(${hue} 45% 32%)`,
      }}
      aria-hidden="true"
    >
      {initials}
    </span>
  );
}

/** Pairs an Avatar with a primary/secondary text line — drop into any table cell or list row. */
export function NameCell({
  name,
  meta,
}: {
  name: ReactNode;
  meta?: ReactNode;
}) {
  return (
    <span className="name-cell">
      <Avatar name={typeof name === "string" ? name : "?"} />
      <span className="name-cell-text">
        <strong>{name}</strong>
        {meta ? <span className="muted">{meta}</span> : null}
      </span>
    </span>
  );
}

/** Row of compact metric cards — replaces bare hero-strip markup with icon + accent styling. */
export function StatStrip({ children }: { children: ReactNode }) {
  return <section className="stat-strip">{children}</section>;
}

export function Stat({
  icon,
  label,
  value,
  tone,
  hint,
}: {
  icon?: ReactNode;
  label: string;
  value: ReactNode;
  tone?: "danger" | "warn" | "success";
  hint?: string;
}) {
  return (
    <div className={`stat-card ${tone ? `stat-card-${tone}` : ""}`}>
      {icon ? <span className="stat-card-icon">{icon}</span> : null}
      <span className="stat-card-body">
        <span className="stat-card-label">{label}</span>
        <span className="stat-card-value">{value}</span>
        {hint ? <span className="stat-card-hint">{hint}</span> : null}
      </span>
    </div>
  );
}

export function EmptyState({
  title,
  body,
  icon,
  action,
}: {
  title: string;
  body: string;
  icon?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <span className="empty-icon">{icon ?? ICONS.inbox}</span>
      <h3>{title}</h3>
      <p>{body}</p>
      {action ? <div style={{ marginTop: "0.75rem" }}>{action}</div> : null}
    </div>
  );
}

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: string; actions?: ReactNode }) {
  return (
    <header className="page-header">
      <div className="page-header-text">
        <span className="page-header-kicker">Operations workspace</span>
        <h1>{title}</h1>
        {subtitle ? <p>{subtitle}</p> : null}
      </div>
      {actions ? <div className="page-header-actions">{actions}</div> : null}
    </header>
  );
}

/** Take photo (rear camera) or upload from gallery — for IMEI stickers, device condition, etc. */
export function PhotoCaptureField({
  label,
  hint,
  previewUrl,
  onFile,
  onClear,
}: {
  label: string;
  hint?: string;
  previewUrl?: string | null;
  onFile: (file: File) => void;
  onClear?: () => void;
}) {
  const cameraRef = useRef<HTMLInputElement>(null);
  const uploadRef = useRef<HTMLInputElement>(null);

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (file) onFile(file);
  }

  return (
    <div className="photo-capture">
      <div className="photo-capture-label">
        <strong>{label}</strong>
        {hint ? <span className="muted">{hint}</span> : null}
      </div>
      <div className="photo-capture-actions">
        <input
          ref={cameraRef}
          type="file"
          accept="image/*"
          capture="environment"
          hidden
          onChange={handleChange}
        />
        <input ref={uploadRef} type="file" accept="image/*" hidden onChange={handleChange} />
        <Button type="button" variant="secondary" onClick={() => cameraRef.current?.click()}>
          Take photo
        </Button>
        <Button type="button" variant="ghost" onClick={() => uploadRef.current?.click()}>
          Upload
        </Button>
        {previewUrl && onClear ? (
          <Button type="button" variant="ghost" onClick={onClear}>
            Remove
          </Button>
        ) : null}
      </div>
      {previewUrl ? (
        <img className="photo-capture-preview" src={previewUrl} alt={label} />
      ) : null}
    </div>
  );
}

/** Non-dismissible wait dialog while STK PIN entry / Daraja Query settles. */
export function StkWaitOverlay({
  visible,
  message = "Waiting for M-Pesa",
  detail = "Ask the customer to enter their PIN on the phone. This closes when payment succeeds.",
  success,
}: {
  visible: boolean;
  message?: string;
  detail?: string;
  success?: string;
}) {
  if (!visible && !success) return null;
  return (
    <div className="stk-wait-overlay" role="alertdialog" aria-modal="true" aria-live="polite">
      <div className="stk-wait-card">
        {success ? (
          <>
            <div className="stk-wait-success" aria-hidden>
              ✓
            </div>
            <strong>{success}</strong>
          </>
        ) : (
          <>
            <div className="stk-wait-spinner" aria-hidden />
            <strong>{message}</strong>
            {detail ? <p className="muted">{detail}</p> : null}
          </>
        )}
      </div>
    </div>
  );
}

export function isTerminalStkError(message: string): boolean {
  const m = message.toLowerCase();
  return ["1032", "1037", "cancelled", "canceled", "ds timeout", "request cancelled"].some((t) => m.includes(t));
}

export function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
