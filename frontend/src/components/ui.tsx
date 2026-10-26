import { useRef, type ButtonHTMLAttributes, type InputHTMLAttributes, type KeyboardEvent, type ReactNode, type SelectHTMLAttributes } from "react";
import { Icon, type IconName } from "./Icon";

type Variant = "track" | "ink" | "ghost";
type Size = "sm" | "md" | "lg";

const variants: Record<Variant, string> = {
  // Track orange is for the one action that matters on a screen: bidding, buying.
  track: "bg-track text-white shadow-[0_6px_14px_-8px_rgb(217_82_10/0.8)] hover:bg-track-dark",
  ink: "bg-ink text-white shadow-[0_6px_14px_-8px_rgb(20_33_61/0.7)] hover:bg-ink/90",
  ghost: "bg-card text-ink border border-line hover:border-ink",
};

const sizes: Record<Size, string> = {
  sm: "min-h-9 rounded-lg px-3 text-sm gap-1.5",
  md: "min-h-11 rounded-lg px-4 gap-2",
  lg: "min-h-13 rounded-xl px-5 font-display text-xl font-bold gap-2 tracking-wide",
};

/** The look of a Button, for links that act as one (a route, not an action). */
export function buttonClass({ variant = "ink", size = "md", className = "" }: { variant?: Variant; size?: Size; className?: string } = {}) {
  return `press relative inline-flex items-center justify-center font-semibold select-none disabled:cursor-not-allowed disabled:opacity-50 ${sizes[size]} ${variants[variant]} ${className}`;
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  icon?: IconName;
  /** Waiting on the server: a spinner covers the label, the width stays put. */
  loading?: boolean;
  /** Just succeeded: the icon becomes a check for as long as the caller keeps this on. */
  done?: boolean;
};

export function Button({ variant = "ink", size = "md", icon, loading = false, done = false, className = "", children, disabled, ...props }: ButtonProps) {
  const iconSize = size === "lg" ? "size-5" : "size-4";
  return (
    <button className={buttonClass({ variant, size, className })} disabled={disabled || loading} aria-busy={loading || undefined} {...props}>
      {/* The label keeps its box while loading, so the button never changes width under the finger. */}
      <span className={`inline-flex items-center justify-center gap-[inherit] transition-opacity duration-150 ${loading ? "opacity-0" : ""}`}>
        {done ? <Icon name="check" className={`${iconSize} pop`} /> : icon && <Icon name={icon} className={iconSize} />}
        {children}
      </span>
      {loading && (
        <span aria-hidden className="absolute inset-0 grid place-items-center">
          <span className={`${iconSize} animate-spin rounded-full border-2 border-current border-r-transparent`} />
        </span>
      )}
    </button>
  );
}

/**
 * One choice out of a few, with a plate that slides to the chosen segment.
 * Segments share the width equally, so the plate moves by whole segments.
 */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
  className = "",
  size = "md",
}: {
  label: string;
  value: T;
  options: { value: T; label: ReactNode }[];
  onChange: (v: T) => void;
  className?: string;
  size?: "md" | "lg";
}) {
  const index = Math.max(0, options.findIndex((o) => o.value === value));
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  // Arrow keys move between segments, as in any native segmented control.
  function onKey(e: KeyboardEvent) {
    const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = (index + step + options.length) % options.length;
    onChange(options[next].value);
    refs.current[next]?.focus();
  }
  return (
    <div
      role="tablist"
      aria-label={label}
      onKeyDown={onKey}
      className={`relative grid auto-cols-fr grid-flow-col border border-line bg-card p-1 ${size === "lg" ? "rounded-2xl" : "rounded-full"} ${className}`}
    >
      <span
        aria-hidden
        className={`segment-plate pointer-events-none absolute inset-y-1 left-1 bg-ink shadow-[0_4px_10px_-6px_rgb(20_33_61/0.8)] ${size === "lg" ? "rounded-xl" : "rounded-full"}`}
        style={{ width: `calc((100% - 0.5rem) / ${options.length})`, translate: `${index * 100}% 0` }}
      />
      {options.map((o, i) => (
        <button
          key={o.value}
          ref={(el) => {
            refs.current[i] = el;
          }}
          type="button"
          role="tab"
          aria-selected={i === index}
          tabIndex={i === index ? 0 : -1}
          onClick={() => onChange(o.value)}
          className={`press relative z-10 font-semibold transition-colors duration-200 ${size === "lg" ? "rounded-xl px-1 py-2.5 sm:px-3" : "rounded-full px-4 py-1.5"} ${i === index ? "text-white" : "text-ink-soft hover:text-ink"}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/**
 * A countdown. In the anti-sniping window each digit that changes rolls in, so
 * the last two minutes read as live; before that it sits still.
 */
export function Clock({ label, closing, className = "" }: { label: string; closing: boolean; className?: string }) {
  if (!closing) return <span className={className}>{label}</span>;
  return (
    <span className={`inline-flex overflow-hidden ${className}`} aria-label={label}>
      {[...label].map((ch, i) => (
        <span key={`${i}${ch}`} aria-hidden className={/\d/.test(ch) ? "roll" : ""}>
          {ch}
        </span>
      ))}
    </span>
  );
}

const control = "w-full rounded-lg border border-line bg-card px-3 py-2.5 text-ink placeholder:text-ink-soft/70 focus:border-ink focus:outline-none";

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-semibold">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-sm text-ink-soft">{hint}</span>}
    </label>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={control} {...props} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={control} {...props} />;
}

export function ErrorNote({ message }: { message: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="rounded-lg bg-signal/10 px-3 py-2 text-sm font-medium text-signal">
      {message}
    </p>
  );
}
