import { comboDeals } from "../data/content";
import { whatsappDirectLink } from "../data/site";

export default function ComboDeals() {
  return (
    <section className="bg-night-950 py-20 text-white">
      <div className="container-page">
        <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-end">
          <div>
            <p className="font-bold text-gold-400">חוסכים יותר</p>
            <h2 className="mt-2 text-3xl font-extrabold sm:text-4xl">כרטיסים משולבים בהנחה</h2>
            <p className="mt-3 max-w-xl text-white/70">
              שילובים חכמים של אטרקציות מובילות במחיר משתלם במיוחד. מושלם למי שרוצה לראות הכל ולחסוך.
            </p>
          </div>
        </div>

        <div className="mt-10 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {comboDeals.map((c) => (
            <a
              key={c.id}
              href={whatsappDirectLink(`היי! אשמח לפרטים על הכרטיס המשולב: ${c.name}`)}
              target="_blank"
              rel="noopener noreferrer"
              className="group overflow-hidden rounded-2xl border border-white/10 bg-white/5 card-hover"
            >
              <div className="relative aspect-[16/10] overflow-hidden">
                <img src={c.image} alt={c.name} loading="lazy" className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110" />
                <span className="absolute right-3 top-3 rounded-full bg-gold-500 px-3 py-1 text-xs font-bold text-night-950">COMBO</span>
              </div>
              <div className="p-4">
                <h3 className="text-base font-bold leading-snug">{c.name}</h3>
                <p className="mt-2 text-sm text-white/60">
                  החל מ-<span className="text-lg font-extrabold text-gold-400">₪{c.fromPrice}</span>
                </p>
              </div>
            </a>
          ))}
        </div>
      </div>
    </section>
  );
}
