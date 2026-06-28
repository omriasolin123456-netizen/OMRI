import { reviews } from "../data/content";
import { StarIcon } from "./icons";

function Stars() {
  return (
    <div className="flex gap-0.5 text-gold-500">
      {Array.from({ length: 5 }).map((_, i) => (
        <StarIcon key={i} className="h-4 w-4" />
      ))}
    </div>
  );
}

export default function Reviews() {
  return (
    <section id="reviews" className="container-page py-20">
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="inline-flex items-center gap-3 rounded-2xl bg-white px-6 py-4 shadow-card">
          <span className="text-4xl font-black text-night-900">5.0</span>
          <div className="text-right">
            <Stars />
            <p className="mt-1 text-sm text-night-900/60">מבוסס על אלפי ביקורות</p>
          </div>
        </div>
        <h2 className="section-title">מה המטיילים שלנו אומרים</h2>
      </div>

      <div className="mt-12 grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3">
        {reviews.map((r) => (
          <figure key={r.name} className="flex flex-col rounded-2xl bg-white p-6 shadow-card">
            <Stars />
            <blockquote className="mt-4 flex-1 text-night-900/80 leading-relaxed">"{r.text}"</blockquote>
            <figcaption className="mt-5 flex items-center justify-between border-t border-night-900/10 pt-4">
              <span className="font-bold text-night-900">{r.name}</span>
              <span className="text-sm text-night-900/50">{r.when}</span>
            </figcaption>
          </figure>
        ))}
      </div>
    </section>
  );
}
