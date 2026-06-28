import { mustDo, topAttractions } from "../data/content";
import AttractionCard from "./AttractionCard";

export default function Attractions() {
  return (
    <section id="attractions" className="container-page py-20">
      <div className="text-center">
        <p className="font-bold text-brand-600">אסור לפספס</p>
        <h2 className="section-title mt-2">אטרקציות חובה בדובאי</h2>
      </div>
      <div className="mt-10 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {mustDo.map((a) => (
          <AttractionCard key={a.id} a={a} />
        ))}
      </div>

      <div className="mt-20 text-center">
        <h2 className="section-title">אטרקציות מובילות בדובאי</h2>
        <p className="mx-auto mt-3 max-w-2xl text-night-900/60">
          המקומות הכי שווים, הכי מצולמים והכי מומלצים על ידי המטיילים שלנו.
        </p>
      </div>
      <div className="mt-10 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {topAttractions.map((a) => (
          <AttractionCard key={a.id} a={a} />
        ))}
      </div>
    </section>
  );
}
