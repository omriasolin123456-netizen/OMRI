import { site } from "../data/site";
import { WhatsAppIcon, StarIcon } from "./icons";

export default function Hero() {
  return (
    <section id="top" className="relative flex min-h-[100svh] items-center overflow-hidden">
      <div className="absolute inset-0">
        <img
          src="https://images.unsplash.com/photo-1512453979798-5ea266f8880c?auto=format&fit=crop&w=2000&q=80"
          alt="קו הרקיע של דובאי"
          className="h-full w-full object-cover"
        />
        <div className="absolute inset-0 bg-gradient-to-b from-night-950/80 via-night-900/60 to-night-950/95" />
      </div>

      <div className="container-page relative z-10 pt-28 pb-16 text-center text-white sm:pt-32">
        <div className="mx-auto max-w-3xl animate-fade-up">
          <span className="inline-flex items-center gap-2 rounded-full border border-gold-400/40 bg-gold-400/10 px-4 py-1.5 text-sm font-semibold text-gold-400">
            <StarIcon className="h-4 w-4" />
            דירוג 5.0 · מבוסס על אלפי מטיילים מרוצים
          </span>

          <h1 className="mt-6 text-4xl font-black leading-tight tracking-tight sm:text-6xl">
            גלו עולם של אטרקציות
            <span className="mt-2 block bg-gradient-to-l from-gold-400 to-gold-600 bg-clip-text text-transparent">
              ופעילויות בדובאי
            </span>
          </h1>

          <p className="mx-auto mt-6 max-w-2xl text-lg text-white/85 sm:text-xl">
            כל הכרטיסים לאטרקציות המובילות בדובאי ואבו דאבי במקום אחד. הזמנה מהירה ופשוטה,
            ייעוץ אישי בעברית, מחירים בשקלים וקבוצת מטיילים שווה.
          </p>

          <div className="mt-9 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <a href={site.whatsappGroupLink} target="_blank" rel="noopener noreferrer" className="btn-whatsapp w-full px-7 py-4 text-lg sm:w-auto">
              <WhatsAppIcon className="h-6 w-6" />
              הצטרפו לקבוצת הוואטסאפ
            </a>
            <a href="#attractions" className="btn-ghost w-full px-7 py-4 text-lg sm:w-auto">
              לכל האטרקציות
            </a>
          </div>

          <dl className="mx-auto mt-12 grid max-w-xl grid-cols-3 gap-4">
            {[
              ["+150", "אטרקציות"],
              ["24/7", "מענה אישי"],
              ["₪", "תשלום בשקלים"],
            ].map(([n, l]) => (
              <div key={l} className="rounded-2xl border border-white/15 bg-white/5 px-3 py-4 backdrop-blur">
                <dt className="text-2xl font-black text-gold-400 sm:text-3xl">{n}</dt>
                <dd className="mt-1 text-sm text-white/80">{l}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </section>
  );
}
