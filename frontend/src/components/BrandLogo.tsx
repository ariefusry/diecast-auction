import { brandLogo } from "../lib/brands";

/** A brand's logo at the given height, or its name when there is no logo. */
export function BrandLogo({ brand, className = "h-5" }: { brand: string; className?: string }) {
  const src = brandLogo[brand];
  if (!src) return <span className="font-semibold">{brand}</span>;
  return <img src={src} alt={brand} className={`w-auto object-contain ${className}`} />;
}
