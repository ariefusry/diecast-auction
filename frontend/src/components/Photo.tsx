import { imageSrc } from "../lib/api";

/** Item photo inside the blister bubble. Fandom's CDN rejects foreign referrers, hence no-referrer. */
export function Photo({ url, alt, className = "" }: { url: string; alt: string; className?: string }) {
  return (
    <div className={`bubble aspect-[4/3] ${className}`}>
      {url ? (
        <img src={imageSrc(url)} alt={alt} referrerPolicy="no-referrer" loading="lazy" className="h-full w-full object-contain p-3 mix-blend-multiply" />
      ) : (
        <div className="flex h-full items-center justify-center text-sm text-ink-soft">Tanpa foto</div>
      )}
    </div>
  );
}
