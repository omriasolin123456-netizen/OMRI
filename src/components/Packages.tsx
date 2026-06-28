import { packages } from "../data/content";
import { whatsappDirectLink } from "../data/site";
import { CheckIcon, WhatsAppIcon } from "./icons";

export default function Packages() {
  return (
    <section id="packages" className="bg-gradient-to-b from-sand-50 to-sand-100 py-20">
      <div className="container-page">
        <div className="text-center">
          <p className="font-bold text-brand-600">הכל מסודר מראש</p>
          <h2 className="section-title mt-2">חבילות ייחודיות</h2>
          <p className="mx-auto mt-3 max-w-2xl text-night-900/60">
            בנינו עבורכם חבילות מנצחות לכל סוג של מטייל. בחרו חבילה ואנחנו נתאים אותה בדיוק עבורכם.
          </p>
        </div>

        <div className="mt-12 grid grid-cols-1 gap-6 lg:grid-cols-3">
          {packages.map((p) => (
            <div
              key={p.id}
              className={`relative flex flex-col overflow-hidden rounded-3xl bg-white shadow-card card-hover ${
                p.highlight ? "ring-2 ring-gold-500" : ""
              }`}
            >
              {p.highlight && (
                <span className="absolute right-5 top-5 z-10 rounded-full bg-gold-500 px-4 py-1 text-sm font-bold text-night-950 shadow">
                  הכי פופולרית
                </span>
              )}
              <div className="relative aspect-[16/9] overflow-hidden">
                <img src={p.image} alt={p.name} loading="lazy" className="h-full w-full object-cover" />
                <div className="absolute inset-0 bg-gradient-to-t from-night-950/70 to-transparent" />
                <h3 className="absolute bottom-4 right-5 text-2xl font-black text-white">{p.name}</h3>
              </div>

              <div className="flex flex-1 flex-col p-6">
                <p className="text-night-900/70">{p.subtitle}</p>
                <ul className="mt-5 flex-1 space-y-2.5">
                  {p.items.map((it) => (
                    <li key={it} className="flex items-start gap-2 text-sm text-night-900/85">
                      <CheckIcon className="mt-0.5 h-5 w-5 shrink-0 text-brand-600" />
                      {it}
                    </li>
                  ))}
                </ul>

                <div className="mt-6 flex items-center justify-between border-t border-night-900/10 pt-5">
                  <div>
                    <span className="block text-xs text-night-900/50">החל מ-</span>
                    <span className="text-2xl font-black text-night-900">₪{p.fromPrice.toLocaleString()}</span>
                  </div>
                  <a
                    href={whatsappDirectLink(`היי! אשמח לפרטים על ${p.name}`)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="btn-whatsapp px-5 py-3 text-sm"
                  >
                    <WhatsAppIcon className="h-5 w-5" />
                    לפרטים
                  </a>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
