import { describe, expect, it } from "vitest";
import { serverError } from "./errors";

describe("serverError", () => {
  it("turns money in a message into rupiah", () => {
    expect(serverError(409, "conflict: bid must be at least 600000")).toBe("Tawaran minimal sekarang Rp600.000.");
    expect(serverError(400, "invalid input: amount must be 1-10000000")).toBe("Jumlah harus Rp1 sampai Rp10.000.000.");
  });
  it("says how to recover", () => {
    expect(serverError(409, "conflict: insufficient balance")).toBe("Saldo tidak cukup. Isi saldo di Dompet lalu coba lagi.");
    expect(serverError(409, "conflict: email or username is taken")).toMatch(/silakan masuk/);
  });
  it("names the field that is too long", () => {
    expect(serverError(400, "invalid input: model_name must be at most 200 characters")).toBe("Nama model maksimal 200 karakter.");
  });
  it("keeps a wrong password apart from an expired session", () => {
    expect(serverError(401, "invalid email or password")).toBe("Email atau kata sandi salah.");
    expect(serverError(401, "unauthorized")).toMatch(/Sesi Anda berakhir/);
  });
  it("never shows English for a message it does not know", () => {
    expect(serverError(409, "conflict: something new")).toBe("Tindakan ini tidak bisa dilakukan sekarang. Muat ulang halaman lalu coba lagi.");
    expect(serverError(502, "")).toBe("Server sedang bermasalah. Coba lagi sebentar lagi.");
  });
});
