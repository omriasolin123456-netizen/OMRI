import { site } from "../data/site";
import { WhatsAppIcon } from "./icons";

export default function WhatsAppBanner() {
  return (
    <section className="relative overflow-hidden bg-[#075E54] py-16 text-white">
      <div
        className="pointer-events-none absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            "radial-gradient(circle at 20% 30%, #25D366 0, transparent 35%), radial-gradient(circle at 80% 70%, #128C7E 0, transparent 40%)",
        }}
      />
      <div className="container-page relative z-10">
        <div className="mx-auto flex max-w-4xl flex-col items-center gap-6 text-center md:flex-row md:text-right">
          <div className="relative shrink-0">
            <span className="absolute inset-0 animate-pulse-ring rounded-full bg-[#25D366]" />
            <span className="relative grid h-24 w-24 place-items-center rounded-full bg-[#25D366] shadow-2xl">
              <WhatsAppIcon className="h-12 w-12 text-white" />
            </span>
          </div>

          <div className="flex-1">
            <h2 className="text-3xl font-black sm:text-4xl">הצטרפו לקבוצת הוואטסאפ שלנו</h2>
            <p className="mt-3 text-lg text-white/85">
              טיפים מעודכנים, המלצות חמות, מענה מהיר ותקשורת עם מטיילים אחרים בדובאי – הכל במקום אחד.
              ההצטרפות חינמית לחלוטין.
            </p>
            <div className="mt-6 flex flex-col items-center gap-3 sm:flex-row md:justify-start">
              <a
                href={site.whatsappGroupLink}
                target="_blank"
                rel="noopener noreferrer"
                className="btn w-full bg-white px-8 py-4 text-lg font-extrabold text-[#075E54] shadow-xl hover:bg-white/90 sm:w-auto"
              >
                <WhatsAppIcon className="h-6 w-6 text-[#25D366]" />
                הצטרפו לקבוצה עכשיו
              </a>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
