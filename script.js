const WHATSAPP_GROUP_URL = "https://chat.whatsapp.com/REPLACE_WITH_YOUR_GROUP_INVITE";

const whatsappLinks = document.querySelectorAll(".js-whatsapp-link");
const navToggle = document.querySelector(".nav-toggle");
const navMenu = document.querySelector("#nav-menu");

whatsappLinks.forEach((link) => {
  link.href = WHATSAPP_GROUP_URL;
  link.setAttribute("aria-label", "הצטרפות לקבוצת הווצאפ של דובאי בקליק");
});

if (navToggle && navMenu) {
  navToggle.addEventListener("click", () => {
    const isOpen = navMenu.classList.toggle("is-open");
    document.body.classList.toggle("menu-open", isOpen);
    navToggle.setAttribute("aria-expanded", String(isOpen));
  });

  navMenu.addEventListener("click", (event) => {
    if (!(event.target instanceof HTMLAnchorElement)) {
      return;
    }

    navMenu.classList.remove("is-open");
    document.body.classList.remove("menu-open");
    navToggle.setAttribute("aria-expanded", "false");
  });
}
