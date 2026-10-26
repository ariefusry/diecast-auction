// The server's clock, as seen from this device. Countdowns subtract "now" from
// an end time the server set; a phone that runs three minutes slow would show
// three minutes too many. Every API response carries the server's Date header,
// so the difference is learned from traffic the page makes anyway.

/**
 * The offset (ms) to add to Date.now() to get server time, given one response.
 * Date has whole-second resolution and is stamped mid-request, so a reading is
 * only good to about a second: changes inside that are kept out, so the clock
 * does not jitter back and forth between polls.
 */
export function clockOffset(dateHeader: string | null, sentAt: number, receivedAt: number, current: number): number {
  const server = Date.parse(dateHeader ?? "");
  if (Number.isNaN(server)) return current;
  // +500: the header truncates to the second, so the true stamp averages half a second later.
  const reading = server + 500 - (sentAt + receivedAt) / 2;
  return Math.abs(reading - current) > 1500 ? Math.round(reading) : current;
}

let offset = 0;

/** Feed one response's Date header into the shared offset. */
export function syncClock(dateHeader: string | null, sentAt: number) {
  offset = clockOffset(dateHeader, sentAt, Date.now(), offset);
}

/** The current time on the server's clock. */
export function serverNow(): number {
  return Date.now() + offset;
}
