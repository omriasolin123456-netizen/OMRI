import { useEffect, useState } from "react";
import { site, whatsappDirectLink } from "../data/site";
import { WhatsAppIcon } from "./icons";

// באנר וואטסאפ צף ודביק - תואם אייפון (env safe-area) ומותאם למובייל ולדסקטופ.
export default function FloatingWhatsApp() {
  const [closed, setClosed] = useState(false);
  const [show, setShow] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setShow(true), 1200);
    return () => clearTimeout(t);
  }, []);

  return (
    <>
      {/* כפתור צ'אט עגול צף - תמיד זמין */}
      <a
        href={whatsappDirectLink()}
        target="_blank"
        rel="noopener noreferrer"
        aria-label="שיחת וואטסאפ"
        className={`fixed left-4 z-50 grid h-14 w-14 place-items-center rounded-full bg-[#25D366] text-white shadow-2xl transition-all duration-300 hover:scale-110 ${
          show ? "opacity-100" : "translate-y-4 opacity-0"
        }`}
        style={{ bottom: "calc(5.5rem + var(--safe-bottom))" }}
      >
        <span className="absolute inset-0 animate-pulse-ring rounded-full bg-[#25D366]" />
        <WhatsAppIcon className="relative h-7 w-7" />
      </a>

      {/* באנר דביק תחתון להצטרפות לקבוצה */}
      {!closed && (
        <div
          className={`fixed inset-x-0 bottom-0 z-50 transition-transform duration-500 ${
            show ? "translate-y-0" : "translate-y-full"
          }`}
          style={{ paddingBottom: "var(--safe-bottom)" }}
        >
          <div className="border-t border-[#128C7E]/40 bg-[#075E54]/95 backdrop-blur">
            <div className="container-page flex items-center gap-3 py-3">
              <span className="hidden h-11 w-11 shrink-0 place-items-center rounded-full bg-[#25D366] text-white sm:grid">
                <WhatsAppIcon className="h-6 w-6" />
              </span>
              <div className="min-w-0 flex-1 text-white">
                <p className="truncate text-sm font-extrabold sm:text-base">הצטרפו לקבוצת הוואטסאפ שלנו</p>
                <p className="truncate text-xs text-white/75">טיפים, המלצות ומענה מהיר – חינם לגמרי</p>
              </div>
              <a
                href={site.whatsappGroupLink}
                target="_blank"
                rel="noopener noreferrer"
                className="shrink-0 rounded-full bg-[#25D366] px-4 py-2.5 text-sm font-bold text-white shadow-lg transition-colors hover:bg-[#1fb955] sm:px-6"
              >
                הצטרפו
              </a>
              <button
                type="button"
                onClick={() => setClosed(true)}
                aria-label="סגירה"
                className="grid h-9 w-9 shrink-0 place-items-center rounded-full text-white/70 transition-colors hover:bg-white/10 hover:text-white"
              >
                <svg viewBox="0 0 20 20" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
                  <path strokeLinecap="round" d="M5 5l10 10M15 5L5 15" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
