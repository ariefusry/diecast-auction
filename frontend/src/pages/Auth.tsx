import { useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { Button, ErrorNote, Field, Input } from "../components/ui";
import { api, errorMessage } from "../lib/api";
import { useAuth } from "../lib/auth";

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const { login } = useAuth();
  const navigate = useNavigate();
  // Sent here from a page that needs an account (a lot you wanted to bid on): go back there after.
  const from = (useLocation().state as { from?: string } | null)?.from;
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const register = mode === "register";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (register) await api("/auth/register", { method: "POST", body: { email, username, password } });
      await login(email, password);
      navigate(from ?? (register ? "/dompet" : "/"), { replace: true });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="mx-auto max-w-sm space-y-4">
      <h1 className="font-display text-4xl font-extrabold italic">{register ? "Daftar" : "Masuk"}</h1>
      <Field label="Email">
        <Input type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
      </Field>
      {register && (
        <Field label="Username" hint="3-30 huruf, angka atau garis bawah.">
          <Input autoComplete="username" required pattern="[A-Za-z0-9_]{3,30}" value={username} onChange={(e) => setUsername(e.target.value)} />
        </Field>
      )}
      <Field label="Password" hint={register ? "Minimal 8 karakter." : undefined}>
        <Input
          type="password"
          autoComplete={register ? "new-password" : "current-password"}
          required
          minLength={register ? 8 : undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </Field>
      <ErrorNote message={error} />
      <Button type="submit" size="lg" loading={busy} className="w-full">
        {register ? "Daftar" : "Masuk"}
      </Button>
      <p className="text-sm text-ink-soft">
        {register ? (
          <>
            Sudah punya akun?{" "}
            <Link to="/masuk" state={{ from }} className="font-semibold text-ink underline underline-offset-4">
              Masuk
            </Link>
          </>
        ) : (
          <>
            Belum punya akun?{" "}
            <Link to="/daftar" state={{ from }} className="font-semibold text-ink underline underline-offset-4">
              Daftar
            </Link>
            .
            {/* The demo password is only known on a dev machine; production sets a random one. */}
            {import.meta.env.DEV && " Untuk mencoba, pakai demo_bidder1@demo.local dengan password demo-password."}
          </>
        )}
      </p>
    </form>
  );
}
