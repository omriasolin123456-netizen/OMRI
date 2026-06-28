import { useState } from "react";
import { faqs } from "../data/content";
import { site, whatsappDirectLink } from "../data/site";
import { ChevronIcon, WhatsAppIcon } from "./icons";

export default function Faq() {
  const [open, setOpen] = useState<number | null>(0);

  return (
    <section id="faq" className="bg-night-950 py-20 text-white">
      <div className="container-page grid gap-12 lg:grid-cols-[1fr_1.2fr]">
        <div>
          <p className="font-bold text-gold-400">סליחה על השאלה</p>
          <h2 className="mt-2 text-3xl font-extrabold sm:text-4xl">שאלות נפוצות</h2>
          <p className="mt-4 text-white/70">
            ריכזנו עבורכם את התשובות לשאלות הנפוצות בתהליך ההזמנה והתשלום. לא מצאתם תשובה?
            אנחנו כאן בשבילכם בכל שאלה, ייעוץ ועזרה בבחירת האטרקציות.
          </p>
          <a href={whatsappDirectLink()} target="_blank" rel="noopener noreferrer" className="btn-whatsapp mt-6">
            <WhatsAppIcon className="h-5 w-5" />
            פנייה בוואטסאפ
          </a>
          <a href={`mailto:${site.email}`} className="mt-3 block text-sm text-white/60 hover:text-white">
            או שלחו לנו מייל: {site.email}
          </a>
        </div>

        <div className="space-y-3">
          {faqs.map((f, i) => {
            const isOpen = open === i;
            return (
              <div key={f.q} className="overflow-hidden rounded-2xl border border-white/10 bg-white/5">
                <button
                  type="button"
                  onClick={() => setOpen(isOpen ? null : i)}
                  className="flex w-full items-center justify-between gap-4 px-5 py-4 text-right"
                >
                  <span className="text-lg font-bold">{f.q}</span>
                  <ChevronIcon className={`h-5 w-5 shrink-0 text-gold-400 transition-transform ${isOpen ? "rotate-180" : ""}`} />
                </button>
                <div className={`grid transition-all duration-300 ${isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}>
                  <div className="overflow-hidden">
                    <p className="px-5 pb-5 text-white/75 leading-relaxed">{f.a}</p>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
