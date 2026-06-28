import { useEffect, useState } from "react";
import { site, whatsappDirectLink } from "../data/site";
import { WhatsAppIcon } from "./icons";

const navLinks = [
  { href: "#categories", label: "קטגוריות" },
  { href: "#attractions", label: "אטרקציות" },
  { href: "#packages", label: "חבילות" },
  { href: "#reviews", label: "המלצות" },
  { href: "#faq", label: "שאלות נפוצות" },
];

export default function Navbar() {
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header
      className={`fixed inset-x-0 top-0 z-40 transition-all duration-300 ${
        scrolled ? "bg-night-950/90 shadow-lg backdrop-blur" : "bg-transparent"
      }`}
      style={{ paddingTop: "var(--safe-top)" }}
    >
      <div className="container-page flex h-16 items-center justify-between gap-4 sm:h-20">
        <a href="#top" className="flex items-center gap-2 text-white">
          <span className="grid h-10 w-10 place-items-center rounded-xl bg-gold-500 text-xl font-black text-night-950">
            ד
          </span>
          <span className="text-lg font-extrabold leading-tight">
            {site.brandName}
            <span className="block text-[11px] font-medium text-gold-400">{site.brandTagline}</span>
          </span>
        </a>

        <nav className="hidden items-center gap-7 lg:flex">
          {navLinks.map((l) => (
            <a key={l.href} href={l.href} className="text-sm font-semibold text-white/85 transition-colors hover:text-gold-400">
              {l.label}
            </a>
          ))}
        </nav>

        <div className="flex items-center gap-2">
          <a href={whatsappDirectLink()} target="_blank" rel="noopener noreferrer" className="btn-whatsapp hidden h-11 px-5 text-sm sm:inline-flex">
            <WhatsAppIcon className="h-5 w-5" />
            דברו איתנו
          </a>
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-label="תפריט"
            className="grid h-11 w-11 place-items-center rounded-xl bg-white/10 text-white lg:hidden"
          >
            <div className="space-y-1.5">
              <span className={`block h-0.5 w-6 bg-white transition-transform ${open ? "translate-y-2 rotate-45" : ""}`} />
              <span className={`block h-0.5 w-6 bg-white transition-opacity ${open ? "opacity-0" : ""}`} />
              <span className={`block h-0.5 w-6 bg-white transition-transform ${open ? "-translate-y-2 -rotate-45" : ""}`} />
            </div>
          </button>
        </div>
      </div>

      {open && (
        <nav className="lg:hidden">
          <div className="container-page space-y-1 bg-night-950/95 pb-4 backdrop-blur">
            {navLinks.map((l) => (
              <a
                key={l.href}
                href={l.href}
                onClick={() => setOpen(false)}
                className="block rounded-lg px-3 py-3 text-base font-semibold text-white/90 hover:bg-white/10"
              >
                {l.label}
              </a>
            ))}
            <a href={whatsappDirectLink()} target="_blank" rel="noopener noreferrer" className="btn-whatsapp mt-2 w-full">
              <WhatsAppIcon className="h-5 w-5" />
              דברו איתנו בוואטסאפ
            </a>
          </div>
        </nav>
      )}
    </header>
  );
}
