import { describe, expect, it } from "vitest";
import { castingNumber, conditionLabel, endsAtWIB, minBid, quickBids, rupiah, timeAgo, timeLeft, trackerStatus, userIdFromToken } from "./format";

describe("rupiah", () => {
  it("formats whole rupiah with dot thousands", () => {
    expect(rupiah(125000)).toBe("Rp125.000");
    expect(rupiah(0)).toBe("Rp0");
  });
});

describe("endsAtWIB", () => {
  it("states the end in Jakarta time whatever the phone's zone", () => {
    // 13:15 UTC is 20:15 WIB on the same Monday.
    expect(endsAtWIB("2026-10-26T13:15:00Z")).toBe("Senin, 26 Okt 20.15 WIB");
    // 18:30 UTC is already Tuesday in Jakarta.
    expect(endsAtWIB("2026-10-26T18:30:00Z")).toBe("Selasa, 27 Okt 01.30 WIB");
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

describe("diorama conditions", () => {
  it("label new and used", () => {
    expect(conditionLabel("used")).toBe("Bekas");
    expect(conditionLabel("new")).toBe("Baru");
  });
});

describe("castingNumber", () => {
  it("splits a Tomica-style number off the name", () => {
    expect(castingNumber("No. 54 Toyota Town Ace")).toEqual({ no: "54", name: "Toyota Town Ace" });
    expect(castingNumber("No.105 Sigma MC74")).toEqual({ no: "105", name: "Sigma MC74" });
  });
  it("leaves other names alone", () => {
    expect(castingNumber("Nissan 300ZX")).toEqual({ no: null, name: "Nissan 300ZX" });
  });
});

describe("timeAgo", () => {
  const now = Date.parse("2026-10-25T10:00:00Z");
  it("reads like a chat timestamp", () => {
    expect(timeAgo("2026-10-25T09:59:50Z", now)).toBe("baru saja");
    expect(timeAgo("2026-10-25T09:57:00Z", now)).toBe("3 mnt lalu");
    expect(timeAgo("2026-10-25T07:00:00Z", now)).toBe("3 jam lalu");
    expect(timeAgo("2026-10-23T10:00:00Z", now)).toBe("2 hari lalu");
  });
});

describe("quickBids", () => {
  it("offers the floor and two bigger jumps", () => {
    expect(quickBids({ start_price: 100, min_increment: 10, current_highest_bid: 150 })).toEqual([160, 170, 200]);
    expect(quickBids({ start_price: 100, min_increment: 10, current_highest_bid: null })).toEqual([100, 110, 140]);
  });
  it("stops at the buy-now price, since a bid there buys the lot", () => {
    expect(quickBids({ start_price: 100, min_increment: 10, current_highest_bid: 150, buy_now_price: 175 })).toEqual([160, 170, 175]);
    expect(quickBids({ start_price: 100, min_increment: 10, current_highest_bid: 150, buy_now_price: 160 })).toEqual([160]);
  });
});

describe("trackerStatus", () => {
  const a = (status: "active" | "completed", leading: boolean, won: boolean) => ({ status, leading, won });
  it("follows the lead while open", () => {
    expect(trackerStatus(a("active", true, false), true)).toBe("Memimpin");
    expect(trackerStatus(a("active", false, false), true)).toBe("Disalip");
  });
  it("gives an ended but unsettled auction to its leader", () => {
    expect(trackerStatus(a("active", true, false), false)).toBe("Menang");
    expect(trackerStatus(a("active", false, false), false)).toBe("Kalah");
  });
  it("reads the winner once settled", () => {
    expect(trackerStatus(a("completed", false, true), false)).toBe("Menang");
    expect(trackerStatus(a("completed", false, false), false)).toBe("Kalah");
  });
});
