// תוכן האתר - אטרקציות, חבילות, קטגוריות, חוות דעת ושאלות נפוצות.
// ניתן לערוך את הטקסטים, המחירים והתמונות בקלות.

export type Category = {
  id: string;
  name: string;
  count: string;
  icon: string;
  image: string;
};

export type Attraction = {
  id: string;
  name: string;
  nameEn: string;
  category: string;
  fromPrice: number;
  image: string;
  tag?: string;
};

export type ComboDeal = {
  id: string;
  name: string;
  fromPrice: number;
  image: string;
};

export type TravelPackage = {
  id: string;
  name: string;
  subtitle: string;
  items: string[];
  fromPrice: number;
  image: string;
  highlight?: boolean;
};

export type Review = {
  name: string;
  text: string;
  when: string;
};

export type FaqItem = {
  q: string;
  a: string;
};

const img = (id: string, w = 800) =>
  `https://images.unsplash.com/${id}?auto=format&fit=crop&w=${w}&q=80`;

export const categories: Category[] = [
  { id: "towers", name: "מגדלים ותצפיות", count: "13 אטרקציות", icon: "🏙️", image: img("photo-1512453979798-5ea266f8880c") },
  { id: "extreme", name: "אקסטרים וספורט", count: "33 אטרקציות", icon: "🏎️", image: img("photo-1583872570712-7b1e0e9b0b6e") },
  { id: "parks", name: "פארקים וגנים", count: "24 אטרקציות", icon: "🌺", image: img("photo-1518684079-3c830dcef090") },
  { id: "shows", name: "מופעים ואמנויות", count: "10 אטרקציות", icon: "🎭", image: img("photo-1503095396549-807759245b35") },
  { id: "tours", name: "סיורים ושייט", count: "36 אטרקציות", icon: "⛵", image: img("photo-1582672060674-bc2bd808a8f5") },
  { id: "museums", name: "מוזיאונים ומונומנטים", count: "11 אטרקציות", icon: "🏛️", image: img("photo-1546412414-e1885259563a") },
  { id: "fun", name: "כיף ומשחקים", count: "20 אטרקציות", icon: "🎢", image: img("photo-1530866495561-507c9faab2ed") },
  { id: "food", name: "אוכל ושתיה", count: "18 אטרקציות", icon: "🍽️", image: img("photo-1414235077428-338989a2e8c0") },
];

export const comboDeals: ComboDeal[] = [
  { id: "c1", name: "בורג' ח'ליפה + תצפית סקיי ויו", fromPrice: 175, image: img("photo-1512453979798-5ea266f8880c") },
  { id: "c2", name: "ספארי במדבר + גן הפרחים", fromPrice: 179, image: img("photo-1451337516015-6b6e9a44a8a3") },
  { id: "c3", name: "שייט בסירות + הכפר הגלובלי", fromPrice: 115, image: img("photo-1582672060674-bc2bd808a8f5") },
  { id: "c4", name: "בורג' ח'ליפה + הגלקסיה של איה", fromPrice: 219, image: img("photo-1546412414-e1885259563a") },
];

export const mustDo: Attraction[] = [
  { id: "a1", name: "גן הפרחים", nameEn: "Miracle Garden", category: "פארקים וגנים", fromPrice: 65, image: img("photo-1518684079-3c830dcef090"), tag: "מומלץ" },
  { id: "a2", name: "מסגרת דובאי", nameEn: "Dubai Frame", category: "מגדלים ותצפיות", fromPrice: 19, image: img("photo-1580674684081-7617fbf3d745") },
  { id: "a3", name: "מוזיאון העתיד", nameEn: "Museum Of The Future", category: "מוזיאונים", fromPrice: 125, image: img("photo-1546412414-e1885259563a"), tag: "חדש" },
  { id: "a4", name: "ספארי במדבר", nameEn: "Desert Adventure", category: "אקסטרים", fromPrice: 135, image: img("photo-1451337516015-6b6e9a44a8a3"), tag: "הכי נמכר" },
];

export const topAttractions: Attraction[] = [
  { id: "t1", name: "מופע לה פרל", nameEn: "La Perle By Dragone", category: "מופעים ואמנויות", fromPrice: 159, image: img("photo-1503095396549-807759245b35") },
  { id: "t2", name: "סיור חצי יום בעברית", nameEn: "Shared Tour in Hebrew", category: "סיורים ושייט", fromPrice: 199, image: img("photo-1526495124232-a04e1849168c") },
  { id: "t3", name: "תצפית אי הדקלים", nameEn: "The View At The Palm", category: "מגדלים ותצפיות", fromPrice: 55, image: img("photo-1518684079-3c830dcef090") },
  { id: "t4", name: "פארק המים אטלנטיס", nameEn: "Aquaventure Waterpark", category: "פארקים", fromPrice: 89, image: img("photo-1530866495561-507c9faab2ed") },
  { id: "t5", name: "שייט בספינת אקסלוסיב", nameEn: "Xclusive Sunset Cruise", category: "אוכל ושתיה", fromPrice: 115, image: img("photo-1582672060674-bc2bd808a8f5") },
  { id: "t6", name: "תצפית הסקיי ויו", nameEn: "Sky Views Observatory", category: "מגדלים ותצפיות", fromPrice: 55, image: img("photo-1512453979798-5ea266f8880c") },
  { id: "t7", name: "השכרת אופנוע ים", nameEn: "Jet Ski Dubai", category: "אקסטרים", fromPrice: 319, image: img("photo-1530053969600-caed2596d242") },
  { id: "t8", name: "הגלקסיה של איה", nameEn: "AYA Universe", category: "מוזיאונים", fromPrice: 99, image: img("photo-1614728263952-84ea256f9679") },
];

export const packages: TravelPackage[] = [
  {
    id: "p-base",
    name: "חבילת בסיס",
    subtitle: "פעם ראשונה בדובאי? 7 האטרקציות המובילות שאתם חייבים לעשות.",
    items: [
      "תצפית אי הדקלים בדובאי",
      "מסגרת דובאי",
      "ספארי במדבר",
      "גן הפרחים",
      "הכפר הגלובלי",
      "סיור חצי יום בעברית",
      "בורג' ח'ליפה",
    ],
    fromPrice: 839,
    image: img("photo-1512453979798-5ea266f8880c", 1000),
  },
  {
    id: "p-couple",
    name: "חבילה זוגית",
    subtitle: "החופשה המושלמת לזוג - אדרנלין וזמן איכות שהוא רק שלכם.",
    items: [
      "תצפית אי הדקלים",
      "גן הפרחים",
      "ספארי במדבר",
      "תצפית הסקיי ויו",
      "מופע לה פרל",
      "שייט בסירות האהבה",
    ],
    fromPrice: 939,
    image: img("photo-1518684079-3c830dcef090", 1000),
    highlight: true,
  },
  {
    id: "p-extreme",
    name: "חבילת אקסטרים",
    subtitle: "הרפתקה וחוויות אדרנלין מהאוויר, מהים ומהיבשה.",
    items: [
      "השכרת אופנוע ים",
      "ספארי במדבר",
      "רכיבה על רייזר במדבר",
      "אומגה Xline מרינה",
      "תצפית הסקיי ויו",
      "שייט בסירות הצהובות",
    ],
    fromPrice: 2129,
    image: img("photo-1530053969600-caed2596d242", 1000),
  },
];

export const reviews: Review[] = [
  {
    name: "מיכל א.",
    text: "ממליצה מאוד על רכישת כרטיסים. מתייחסים מהר, נותנים את כל המידע לכל אטרקציה והכרטיסים מגיעים כמעט מיידית למייל. ממש נוח ויעיל!",
    when: "לפני 4 חודשים",
  },
  {
    name: "דניאל ל.",
    text: "מרגע ההזמנה הנוחה ועד לטיול החוויתי הכל היה מושלם. שאלות נענו מיידית בוואטסאפ. לקחנו מסלול פרימיום למשפחה וזו הייתה החלטה נבונה.",
    when: "לפני 4 חודשים",
  },
  {
    name: "רונית ש.",
    text: "השירות כולל מענה זמין ומיידי בקבוצות בוואטסאפ. הסוכן האישי ליווה אותנו 24/7, החל מחודש וחצי לפני הטיול. ממליצה בחום!",
    when: "לפני 5 חודשים",
  },
  {
    name: "יוסי ב.",
    text: "החברה הכי מקצועית לטיול בדובאי. קיבלנו טיפים מצוינים, גם כאלה שלא לרכוש מראש - מה שמראה שטובת המטייל קודמת. מחירים הכי טובים שיש.",
    when: "לפני 5 חודשים",
  },
  {
    name: "אריאלה כ.",
    text: "דובאי מקום מהמם עם המון אטרקציות. התכנון המשותף היה חוויה, המלצות מדויקות איזה אטרקציות וזמנים מתאימים לנו. תודה ענקית!",
    when: "לפני 5 חודשים",
  },
  {
    name: "ענת מ.",
    text: "מענה מהיר בוואטסאפ ואדיבות אינסופית. הקבוצה של המטיילים עזרה לנו המון לתקשר ולתאם. לא לנסוע לדובאי בלי זה!",
    when: "לפני 6 חודשים",
  },
];

export const faqs: FaqItem[] = [
  {
    q: "איך מזמינים כרטיסים?",
    a: "פשוט בוחרים את האטרקציה הרצויה, פונים אלינו בוואטסאפ או במייל ומקבלים פרטים מלאים, ייעוץ אישי וקישור לתשלום מאובטח. הכרטיסים נשלחים ישירות למייל ולנייד.",
  },
  {
    q: "האם המחירים בשקלים?",
    a: "כן, החיוב מתבצע בשקלים (₪) כך שאין הפתעות בכרטיס האשראי ואין צורך בהמרת מטבע.",
  },
  {
    q: "מתי כדאי להזמין את האטרקציות?",
    a: "מומלץ להזמין מראש כדי להבטיח מקום, במיוחד באטרקציות פופולריות ובחגים. אנחנו נשמח לבנות לכם לוח זמנים מסודר לכל ימי הטיול.",
  },
  {
    q: "מה כוללת קבוצת הוואטסאפ?",
    a: "הקבוצה היא מקור עצום של מידע - טיפים מעודכנים, המלצות, מענה מהיר לשאלות ותקשורת עם מטיילים אחרים שנמצאים בדובאי באותו זמן.",
  },
  {
    q: "האם יש אפשרות לאטרקציות וטיולים כשרים?",
    a: "בהחלט. אנחנו מציעים ספארי כשר, שייט כשר ואפשרויות אוכל כשר. ספרו לנו מה אתם צריכים ונתאים לכם את הטיול.",
  },
];
