// Thin client for the Go API. Errors carry the server's {"error": ...} message.

export const API_URL: string = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export type Item = {
  id: number;
  brand: string;
  model_name: string;
  series: string;
  year: number;
  color: string;
  tags: string[];
  condition: string;
  image_url: string;
  source_url: string;
  description: string;
};

export type Auction = {
  id: number;
  seller_id: number;
  item: Item;
  start_price: number;
  min_increment: number;
  buy_now_price: number | null;
  current_highest_bid: number | null;
  highest_bidder_id: number | null;
  status: "draft" | "active" | "completed" | "cancelled";
  duration_minutes: number;
  start_time: string | null;
  end_time: string | null;
  winner_id: number | null;
};

export type Filter = {
  brand: string;
  series: string;
  color: string;
  tags: string[] | null;
  condition: string;
  min_price: number;
  max_price: number;
  text: string;
  sort: string;
};

export type SearchResult = { ai: boolean; relaxed: boolean; filter: Filter; auctions: Auction[] };
export type Wallet = { available_balance: number; locked_balance: number };

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : "Terjadi kesalahan. Coba lagi.";
}

const TOKEN_KEY = "token";

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string | null) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    // Storage blocked (private mode): the session just won't survive a reload.
  }
}

export async function api<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  let body: BodyInit | undefined;
  if (init.body instanceof FormData) body = init.body;
  else if (init.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(init.body);
  }

  let res: Response;
  try {
    res = await fetch(API_URL + path, { method: init.method ?? "GET", headers, body });
  } catch {
    throw new ApiError(0, "Server tidak bisa dihubungi. Periksa koneksi lalu coba lagi.");
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    // A wrong password on /auth/login is also a 401; that must not end the session.
    if (res.status === 401 && token && !path.startsWith("/auth/")) {
      setToken(null); // expired or revoked session
      window.dispatchEvent(new Event("auth:expired"));
    }
    throw new ApiError(res.status, data.error ?? `HTTP ${res.status}`);
  }
  return data as T;
}

/** Photo URL for <img>: uploads live on the API host, wiki photos are absolute. */
export function imageSrc(url: string): string {
  return url.startsWith("/") ? API_URL + url : url;
}

export type BidEntry = {
  bidder_id: number;
  bidder: string; // masked, e.g. d***1
  amount: number;
  created_at: string;
};
