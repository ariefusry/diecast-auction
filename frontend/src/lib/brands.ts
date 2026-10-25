// Logos sit in public/brands: Hot Wheels, Mini GT, Matchbox and Tomica are
// public-domain text logos from Wikimedia Commons; the rest are the brand
// logos the Fandom wikis use. Trademarks belong to their owners.
export const brandLogo: Record<string, string> = {
  "Hot Wheels": "/brands/hot-wheels.svg",
  "Mini GT": "/brands/mini-gt.svg",
  Inno64: "/brands/inno64.png",
  Matchbox: "/brands/matchbox.svg",
  Tomica: "/brands/tomica.png",
  "Tarmac Works": "/brands/tarmac-works.png",
  "M2 Machines": "/brands/m2-machines.png",
};

// The card-back color each brand prints on its packaging, so a collector
// can tell brands apart from across the room as they would on a shop peg.
export const brandBack: Record<string, string> = {
  "Hot Wheels": "#1d5bbf",
  "Mini GT": "#1c1c1e",
  Inno64: "#b3121d",
  Matchbox: "#f39a00",
  Tomica: "#e1001a",
  "Tarmac Works": "#7b8b63",
  "M2 Machines": "#5c1218",
};
