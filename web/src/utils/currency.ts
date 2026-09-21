export const COMMON_CURRENCIES = [
  { value: "USD", label: "USD — US Dollar ($)" },
  { value: "CNY", label: "CNY — Chinese Yuan (¥)" },
  { value: "JPY", label: "JPY — Japanese Yen (¥)" },
  { value: "CAD", label: "CAD — Canadian Dollar (C$)" },
  { value: "EUR", label: "EUR — Euro (€)" },
  { value: "GBP", label: "GBP — British Pound (£)" },
  { value: "HKD", label: "HKD — Hong Kong Dollar (HK$)" },
  { value: "AUD", label: "AUD — Australian Dollar (A$)" },
  { value: "SGD", label: "SGD — Singapore Dollar (S$)" },
  { value: "KRW", label: "KRW — South Korean Won (₩)" },
  { value: "TWD", label: "TWD — New Taiwan Dollar (NT$)" },
  { value: "CHF", label: "CHF — Swiss Franc (CHF)" },
  { value: "RUB", label: "RUB — Russian Ruble (₽)" },
  { value: "INR", label: "INR — Indian Rupee (₹)" },
  { value: "VND", label: "VND — Vietnamese Dong (₫)" },
  { value: "THB", label: "THB — Thai Baht (฿)" },
] as const;

const LEGACY_SYMBOLS: Record<string, string> = {
  $: "USD",
  "€": "EUR",
  "£": "GBP",
  "₽": "RUB",
  "₹": "INR",
  "₫": "VND",
  "฿": "THB",
};

export function normalizeCurrencyInput(value: string): string {
  const raw = value.trim();
  if (!raw) return "USD";
  if (raw === "¥" || raw === "￥") return "CNY";
  if (LEGACY_SYMBOLS[raw]) return LEGACY_SYMBOLS[raw];
  const code = raw.toUpperCase();
  return code === "RMB" ? "CNY" : code;
}

export function displayCurrencyCode(value?: string): string {
  return normalizeCurrencyInput(value || "USD");
}
