// =====================================================================
//  הגדרות האתר - ערוך כאן את הפרטים האישיים שלך
//  Edit your personal details here. No coding knowledge needed.
// =====================================================================

export const site = {
  // שם המותג שמופיע בראש האתר ובכותרות
  brandName: "דובאי טיקטס",
  brandTagline: "כרטיסים לאטרקציות בדובאי",

  // מספר הוואטסאפ שלך בפורמט בינלאומי ללא + וללא רווחים
  // לדוגמה למספר ישראלי 050-123-4567 -> "972501234567"
  whatsappNumber: "971500000000",

  // קישור לקבוצת הוואטסאפ שלך (מקבלים אותו דרך וואטסאפ -> פרטי קבוצה -> הזמן באמצעות קישור)
  whatsappGroupLink: "https://chat.whatsapp.com/XXXXXXXXXXXXXXXXXXXXXX",

  // הודעה שתופיע אוטומטית כשפותחים צ'אט וואטסאפ אישי
  whatsappPrefilledMessage:
    "היי! הגעתי מהאתר ואשמח לקבל פרטים על אטרקציות בדובאי 😊",

  // אימייל ליצירת קשר
  email: "info@dubai-tickets.example",

  // קישורים לרשתות חברתיות (השאר ריק כדי להסתיר)
  instagram: "https://instagram.com/",
  facebook: "https://facebook.com/",
};

// בונה קישור לצ'אט וואטסאפ אישי עם הודעה מוכנה
export function whatsappDirectLink(text?: string) {
  const msg = encodeURIComponent(text ?? site.whatsappPrefilledMessage);
  return `https://wa.me/${site.whatsappNumber}?text=${msg}`;
}
