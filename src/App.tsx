import Navbar from "./components/Navbar";
import Hero from "./components/Hero";
import Categories from "./components/Categories";
import ComboDeals from "./components/ComboDeals";
import Attractions from "./components/Attractions";
import Packages from "./components/Packages";
import WhatsAppBanner from "./components/WhatsAppBanner";
import Reviews from "./components/Reviews";
import Faq from "./components/Faq";
import Footer from "./components/Footer";
import FloatingWhatsApp from "./components/FloatingWhatsApp";

export default function App() {
  return (
    <div className="min-h-screen bg-sand-50">
      <Navbar />
      <main>
        <Hero />
        <Categories />
        <ComboDeals />
        <Attractions />
        <Packages />
        <WhatsAppBanner />
        <Reviews />
        <Faq />
      </main>
      <Footer />
      <FloatingWhatsApp />
    </div>
  );
}
