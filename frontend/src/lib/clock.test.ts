import { describe, expect, it } from "vitest";
import { clockOffset } from "./clock";

describe("clockOffset", () => {
  // The phone thinks it is 10:00:00.000; the request took 200ms.
  const sent = Date.parse("2026-10-26T10:00:00.000Z");
  const got = sent + 200;

  it("learns a phone that runs 3 minutes slow", () => {
    // Server stamped 10:03:00 (whole second) mid-request.
    const off = clockOffset("Mon, 26 Oct 2026 10:03:00 GMT", sent, got, 0);
    expect(off).toBeGreaterThan(179_000);
    expect(off).toBeLessThan(181_000);
  });
  it("keeps the current offset for sub-second jitter", () => {
    expect(clockOffset("Mon, 26 Oct 2026 10:00:00 GMT", sent, got, 300)).toBe(300);
  });
  it("ignores a missing or broken Date header", () => {
    expect(clockOffset(null, sent, got, 42)).toBe(42);
    expect(clockOffset("not a date", sent, got, 42)).toBe(42);
  });
});
