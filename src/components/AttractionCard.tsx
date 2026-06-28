import type { Attraction } from "../data/content";
import { whatsappDirectLink } from "../data/site";

export default function AttractionCard({ a }: { a: Attraction }) {
  const link = whatsappDirectLink(`היי! אשמח לפרטים ולהזמנה של: ${a.name} (${a.nameEn})`);
  return (
    <a
      href={link}
      target="_blank"
      rel="noopener noreferrer"
      className="group card-hover block overflow-hidden rounded-2xl bg-white shadow-card"
    >
      <div className="relative aspect-[4/3] overflow-hidden">
        <img
          src={a.image}
          alt={a.name}
          loading="lazy"
          className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110"
        />
        <div className="absolute inset-0 bg-gradient-to-t from-night-950/60 via-transparent to-transparent" />
        {a.tag && (
          <span className="absolute right-3 top-3 rounded-full bg-gold-500 px-3 py-1 text-xs font-bold text-night-950 shadow">
            {a.tag}
          </span>
        )}
      </div>
      <div className="p-4">
        <p className="text-xs font-semibold text-brand-600">{a.category}</p>
        <h3 className="mt-1 text-lg font-bold leading-snug text-night-900">{a.name}</h3>
        <p className="text-sm text-night-900/50">{a.nameEn}</p>
        <div className="mt-3 flex items-center justify-between">
          <span className="text-sm text-night-900/60">
            החל מ-<span className="text-xl font-extrabold text-night-900">₪{a.fromPrice}</span>
          </span>
          <span className="rounded-full bg-brand-500/10 px-3 py-1.5 text-sm font-bold text-brand-700 transition-colors group-hover:bg-brand-500 group-hover:text-white">
            לפרטים
          </span>
        </div>
      </div>
    </a>
  );
}
