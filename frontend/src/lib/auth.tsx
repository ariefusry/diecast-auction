import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api, getToken, setToken } from "./api";
import { userIdFromToken } from "./format";

type Auth = {
  userId: number | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<Auth | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [userId, setUserId] = useState(() => {
    const t = getToken();
    return t ? userIdFromToken(t) : null;
  });

  const login = useCallback(async (email: string, password: string) => {
    const { token } = await api<{ token: string }>("/auth/login", { method: "POST", body: { email, password } });
    setToken(token);
    setUserId(userIdFromToken(token));
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setUserId(null);
  }, []);

  useEffect(() => {
    const expired = () => setUserId(null);
    window.addEventListener("auth:expired", expired);
    return () => window.removeEventListener("auth:expired", expired);
  }, []);

  return <AuthContext.Provider value={{ userId, login, logout }}>{children}</AuthContext.Provider>;
}

export function useAuth(): Auth {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside AuthProvider");
  return ctx;
}
