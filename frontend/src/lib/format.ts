const thousands = new Intl.NumberFormat("id-ID");

/** Whole rupiah, as written on Indonesian price tags: Rp125.000 */
export function rupiah(n: number): string {
  return "Rp" + thousands.format(n);
}

const pad = (n: number) => String(n).padStart(2, "0");

/**
 * Time until endIso at nowMs. `closing` marks the final 2 minutes, where a
 * bid extends the auction by 2 minutes (anti-sniping).
 */
export function timeLeft(endIso: string | null, nowMs: number): { ms: number; label: string; closing: boolean } {
  if (!endIso) return { ms: 0, label: "Belum tayang", closing: false };
  const ms = Math.max(0, Date.parse(endIso) - nowMs);
  if (ms === 0) return { ms, label: "Selesai", closing: false };

  const s = Math.floor(ms / 1000);
  const days = Math.floor(s / 86400);
  const hours = Math.floor((s % 86400) / 3600);
  const mins = Math.floor((s % 3600) / 60);
  const secs = s % 60;
  let label: string;
  if (days > 0) label = `${days}h ${hours}j`;
  else if (hours > 0) label = `${hours}j ${pad(mins)}m ${pad(secs)}d`;
  else label = `${pad(mins)}:${pad(secs)}`;
  return { ms, label, closing: ms <= 120_000 };
}

/** The lowest bid the server will accept right now. */
export function minBid(a: { start_price: number; min_increment: number; current_highest_bid: number | null }): number {
  return a.current_highest_bid === null ? a.start_price : a.current_highest_bid + a.min_increment;
}

export const conditions: Record<string, string> = {
  mint_on_card: "Mint on card",
  sealed_box: "Box segel",
  acrylic_case: "Akrilik",
  loose_mint: "Loose mulus",
  damaged_packaging: "Kemasan rusak",
};

/** Dioramas are sold new (by their artist) or used (by an owner). */
export const dioramaConditions: Record<string, string> = {
  new: "Baru",
  used: "Bekas",
};

export function conditionLabel(c: string): string {
  return conditions[c] ?? dioramaConditions[c] ?? c;
}

/** The user ID (JWT subject) of a still-valid token, or null. */
export function userIdFromToken(token: string): number | null {
  try {
    const part = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    const claims = JSON.parse(atob(part.padEnd(part.length + ((4 - (part.length % 4)) % 4), "=")));
    if (typeof claims.exp === "number" && claims.exp * 1000 < Date.now()) return null;
    const id = Number(claims.sub);
    return Number.isInteger(id) && id > 0 ? id : null;
  } catch {
    return null;
  }
}

/** Tomica prints a casting number on every pack ("No. 54"); split it off the name for a badge. */
export function castingNumber(name: string): { no: string | null; name: string } {
  const m = /^No\.\s*(\d+)\s+(.+)$/.exec(name);
  return m ? { no: m[1], name: m[2] } : { no: null, name };
}

/** A bid's age, the way a chat app writes it. */
export function timeAgo(iso: string, nowMs: number): string {
  const s = Math.max(0, Math.floor((nowMs - Date.parse(iso)) / 1000));
  if (s < 60) return "baru saja";
  if (s < 3600) return `${Math.floor(s / 60)} mnt lalu`;
  if (s < 86400) return `${Math.floor(s / 3600)} jam lalu`;
  return `${Math.floor(s / 86400)} hari lalu`;
}

/**
 * My standing in a tracked auction. Until the closer settles it, an auction
 * whose time is up has no winner yet (won is false), so its leader is the one who wins.
 */
export function trackerStatus(
  a: { status: string; leading: boolean; won: boolean },
  open: boolean,
): "Memimpin" | "Disalip" | "Menang" | "Kalah" {
  const mine = a.status === "active" ? a.leading : a.won;
  if (open) return mine ? "Memimpin" : "Disalip";
  return mine ? "Menang" : "Kalah";
}

/** One-tap amounts: the minimum bid, one step more, and a five-step jump. */
export function quickBids(a: { start_price: number; min_increment: number; current_highest_bid: number | null }): number[] {
  const floor = minBid(a);
  return [floor, floor + a.min_increment, floor + 4 * a.min_increment];
}
