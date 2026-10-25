// Logos sit in public/brands: Hot Wheels, Mini GT, Matchbox and Tomica are
// public-domain text logos from Wikimedia Commons; the rest are the brand
// logos the Fandom wikis use. Trademarks belong to their owners.
const logos: Record<string, string> = {
  "Hot Wheels": "/brands/hot-wheels.svg",
  "Mini GT": "/brands/mini-gt.svg",
  Inno64: "/brands/inno64.png",
  Matchbox: "/brands/matchbox.svg",
  Tomica: "/brands/tomica.png",
  "Tarmac Works": "/brands/tarmac-works.png",
  "M2 Machines": "/brands/m2-machines.png",
};

/** A brand's logo at the given height, or its name when there is no logo. */
export function BrandLogo({ brand, className = "h-5" }: { brand: string; className?: string }) {
  const src = logos[brand];
  if (!src) return <span className="font-semibold">{brand}</span>;
  return <img src={src} alt={brand} className={`w-auto object-contain ${className}`} />;
}
