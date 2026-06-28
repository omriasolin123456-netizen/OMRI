# Dubai Landing Site

אתר נחיתה סטטי בעברית לדובאי: אטרקציות, חבילות, FAQ ובאנר ווצאפ רספונסיבי שמותאם גם לאייפון.

## קבצים עיקריים

- `index.html` - מבנה האתר והתוכן בעברית.
- `styles.css` - עיצוב מלא, RTL, רספונסיביות ו-safe-area לאייפון.
- `script.js` - תפריט מובייל וקישור ווצאפ מרכזי.

## החלפת קישור קבוצת הווצאפ

פתחו את `script.js` והחליפו את הערך:

```js
const WHATSAPP_GROUP_URL = "https://chat.whatsapp.com/REPLACE_WITH_YOUR_GROUP_INVITE";
```

בקישור ההזמנה האמיתי של קבוצת הווצאפ שלכם.

## הפעלה מקומית

אפשר לפתוח את `index.html` ישירות בדפדפן, או להריץ שרת סטטי:

```bash
python3 -m http.server 4173
```

ואז להיכנס לכתובת:

```text
http://localhost:4173
```
