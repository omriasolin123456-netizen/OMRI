import { site, whatsappDirectLink } from "../data/site";
import { WhatsAppIcon } from "./icons";

export default function Footer() {
  const year = new Date().getFullYear();
  return (
    <footer className="bg-night-900 pt-16 text-white" style={{ paddingBottom: "calc(5rem + var(--safe-bottom))" }}>
      <div className="container-page grid gap-10 sm:grid-cols-2 lg:grid-cols-4">
        <div className="sm:col-span-2 lg:col-span-1">
          <div className="flex items-center gap-2">
            <span className="grid h-10 w-10 place-items-center rounded-xl bg-gold-500 text-xl font-black text-night-950">ד</span>
            <span className="text-lg font-extrabold">{site.brandName}</span>
          </div>
          <p className="mt-4 max-w-xs text-sm text-white/60">
            אתר הזמנת הכרטיסים לאטרקציות המובילות בדובאי ואבו דאבי. ייעוץ אישי בעברית, מחירים בשקלים ושירות מכל הלב.
          </p>
        </div>

        <div>
          <h4 className="font-bold text-gold-400">ניווט מהיר</h4>
          <ul className="mt-4 space-y-2 text-sm text-white/70">
            <li><a href="#categories" className="hover:text-white">קטגוריות</a></li>
            <li><a href="#attractions" className="hover:text-white">אטרקציות</a></li>
            <li><a href="#packages" className="hover:text-white">חבילות</a></li>
            <li><a href="#reviews" className="hover:text-white">המלצות</a></li>
            <li><a href="#faq" className="hover:text-white">שאלות נפוצות</a></li>
          </ul>
        </div>

        <div>
          <h4 className="font-bold text-gold-400">יצירת קשר</h4>
          <ul className="mt-4 space-y-3 text-sm text-white/70">
            <li>
              <a href={whatsappDirectLink()} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 hover:text-white">
                <WhatsAppIcon className="h-4 w-4 text-[#25D366]" /> וואטסאפ אישי
              </a>
            </li>
            <li>
              <a href={site.whatsappGroupLink} target="_blank" rel="noopener noreferrer" className="hover:text-white">קבוצת הוואטסאפ</a>
            </li>
            <li><a href={`mailto:${site.email}`} className="hover:text-white">{site.email}</a></li>
          </ul>
        </div>

        <div>
          <h4 className="font-bold text-gold-400">עקבו אחרינו</h4>
          <ul className="mt-4 space-y-2 text-sm text-white/70">
            {site.instagram && <li><a href={site.instagram} target="_blank" rel="noopener noreferrer" className="hover:text-white">אינסטגרם</a></li>}
            {site.facebook && <li><a href={site.facebook} target="_blank" rel="noopener noreferrer" className="hover:text-white">פייסבוק</a></li>}
          </ul>
        </div>
      </div>

      <div className="mt-12 border-t border-white/10">
        <div className="container-page py-6 text-center text-xs text-white/50">
          © כל הזכויות שמורות {year} · {site.brandName} | ט.ל.ח
        </div>
      </div>
    </footer>
  );
}
