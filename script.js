const yearNode = document.getElementById("year");
if (yearNode) {
  yearNode.textContent = new Date().getFullYear();
}

document.querySelector("form")?.addEventListener("submit", (event) => {
  event.preventDefault();
  alert("תודה! קיבלנו את הפרטים ונחזור אליכם בהקדם.");
});
