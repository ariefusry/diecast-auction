import { describe, expect, it } from "vitest";
import { conditionLabel, minBid, rupiah, timeLeft, userIdFromToken } from "./format";

describe("rupiah", () => {
  it("formats whole rupiah with dot thousands", () => {
    expect(rupiah(125000)).toBe("Rp125.000");
    expect(rupiah(0)).toBe("Rp0");
  });
});

describe("timeLeft", () => {
  const end = "2026-10-25T10:00:00Z";
  const at = (iso: string) => Date.parse(iso);

  it("counts down with hours, minutes and seconds", () => {
    expect(timeLeft(end, at("2026-10-25T07:58:05Z"))).toEqual({ ms: 7315000, label: "2j 01m 55d", closing: false });
  });
  it("shows days for long auctions", () => {
    expect(timeLeft(end, at("2026-10-23T09:00:00Z")).label).toBe("2h 1j");
  });
  it("flags the anti-sniping window under 2 minutes", () => {
    expect(timeLeft(end, at("2026-10-25T09:58:30Z"))).toEqual({ ms: 90000, label: "01:30", closing: true });
  });
  it("is over at end_time", () => {
    expect(timeLeft(end, at("2026-10-25T10:00:01Z"))).toEqual({ ms: 0, label: "Selesai", closing: false });
  });
  it("handles a draft without end_time", () => {
    expect(timeLeft(null, 0).label).toBe("Belum tayang");
  });
});

describe("minBid", () => {
  it("is the start price before any bid, then leader plus increment", () => {
    expect(minBid({ start_price: 100000, min_increment: 10000, current_highest_bid: null })).toBe(100000);
    expect(minBid({ start_price: 100000, min_increment: 10000, current_highest_bid: 130000 })).toBe(140000);
  });
});

describe("conditionLabel", () => {
  it("names conditions the way collectors say them", () => {
    expect(conditionLabel("mint_on_card")).toBe("Mint on card");
    expect(conditionLabel("damaged_packaging")).toBe("Kemasan rusak");
  });
});

describe("userIdFromToken", () => {
  it("reads the subject of a JWT", () => {
    const payload = btoa(JSON.stringify({ sub: "42", exp: 9999999999 })).replace(/=+$/, "");
    expect(userIdFromToken(`h.${payload}.s`)).toBe(42);
  });
  it("returns null for junk or expired tokens", () => {
    expect(userIdFromToken("junk")).toBeNull();
    const old = btoa(JSON.stringify({ sub: "42", exp: 1 }));
    expect(userIdFromToken(`h.${old}.s`)).toBeNull();
  });
});
