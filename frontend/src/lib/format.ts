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

export function conditionLabel(c: string): string {
  return conditions[c] ?? c;
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
