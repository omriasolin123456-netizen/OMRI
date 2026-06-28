import { categories } from "../data/content";
import { whatsappDirectLink } from "../data/site";

export default function Categories() {
  return (
    <section id="categories" className="container-page py-20">
      <div className="text-center">
        <p className="font-bold text-brand-600">בחרו לפי תחום עניין</p>
        <h2 className="section-title mt-2">קטגוריות מובילות</h2>
        <p className="mx-auto mt-3 max-w-2xl text-night-900/60">
          דובאי – גן עדן שופע אטרקציות, בילויים וכיף שאינו נגמר. בחרו את הקטגוריה שמדברת אליכם.
        </p>
      </div>

      <div className="mt-12 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        {categories.map((c) => (
          <a
            key={c.id}
            href={whatsappDirectLink(`היי! אשמח לפרטים על אטרקציות בקטגוריה: ${c.name}`)}
            target="_blank"
            rel="noopener noreferrer"
            className="group relative overflow-hidden rounded-2xl shadow-card card-hover"
          >
            <div className="aspect-[4/5] sm:aspect-square">
              <img src={c.image} alt={c.name} loading="lazy" className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110" />
            </div>
            <div className="absolute inset-0 bg-gradient-to-t from-night-950/90 via-night-950/30 to-transparent" />
            <div className="absolute inset-x-0 bottom-0 p-4 text-white">
              <span className="text-2xl">{c.icon}</span>
              <h3 className="mt-1 text-lg font-bold leading-tight">{c.name}</h3>
              <p className="text-sm text-gold-400">{c.count}</p>
            </div>
          </a>
        ))}
      </div>
    </section>
  );
}
