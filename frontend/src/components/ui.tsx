import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";

type Variant = "track" | "ink" | "ghost";

const variants: Record<Variant, string> = {
  track: "bg-track text-white hover:bg-track-dark",
  ink: "bg-ink text-white hover:bg-ink/90",
  ghost: "bg-transparent text-ink border border-line hover:border-ink",
};

export function Button({ variant = "ink", className = "", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2.5 font-semibold disabled:cursor-not-allowed disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    />
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
