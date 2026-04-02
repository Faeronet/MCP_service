/**
 * Латинские имена ангелов из страниц (validationName) → русская транскрипция (как в списке 1–72).
 */
export const ANGEL_NAME_LATIN_TO_RU = {
  Achaiah: 'Ахаиах',
  Aladiah: 'Аладиах',
  Anauel: 'Анаюель',
  Aniel: 'Аниель',
  Ariel: 'Ариель',
  Cahetel: 'Кахетель',
  Caliel: 'Калиель',
  Damabiah: 'Дамабиах',
  Daniel: 'Даниель',
  Elemiah: 'Элемиах',
  Eyael: 'Эйаёль',
  Haaiah: 'Хааиах',
  Haamiah: 'Хаамиах',
  Habuhiah: 'Хабюиах',
  Hahahel: 'Хахахель',
  Hahaiah: 'Хахаиах',
  Hahasiah: 'Хахасиах',
  Haiaiel: 'Хаиаиель',
  Harahel: 'Харахель',
  Hariel: 'Хариель',
  Haziel: 'Хазиель',
  Hekamiah: 'Хакамиах',
  Iahhel: 'Иаххель',
  Ieiazel: 'Иейазель',
  Iezalel: 'Иезелель',
  Imamiah: 'Имамиах',
  Jabamiah: 'Иабамиах',
  Jeliel: 'Иелиель',
  Khavakhiah: 'Кавакиах',
  Lauviah: 'Лауиах',
  Lehahiah: 'Лехахиах',
  Lelahel: 'Лелахель',
  Leuviah: 'Леувиах',
  Leviah: 'Левиах',
  Lecabel: 'Лекабель',
  Mahasiah: 'Махазиах',
  Manakel: 'Манакель',
  Mebahel: 'Мебахель',
  Mebahiah: 'Мебаиах',
  Melahel: 'Мелахель',
  Menadel: 'Манадель',
  Mihael: 'Михаёль',
  Mikael: 'Микаёль',
  Mitzrael: 'Мизраель',
  Mumiah: 'Мюмиах',
  Nanael: 'Нанаёль',
  Nelkhael: 'Нелькаель',
  Nemamiah: 'Неммамиах',
  Nithael: 'Нитаёль',
  'Nith-Haiah': 'Нитхаиах',
  Omael: 'Ормаёль',
  OmaDamabiahel: 'Ормаёль',
  Pahaliah: 'Пахалиах',
  Poyel: 'Поиель',
  Rehael: 'Рехаёль',
  Reiyel: 'Рейиель',
  Rochel: 'Рохель',
  Sealiah: 'Сеалиах',
  Seheiah: 'Сехиах',
  Sitael: 'Ситаель',
  Umabel: 'Умабель',
  Vasariah: 'Васариах',
  Vehuel: 'Вехюель',
  Vehuiah: 'Вехюиах',
  Veuliah: 'Вевалиах',
  Yehuiah: 'Иехюиах',
  Yeialel: 'Иеиалель',
  Yeiayel: 'Иеиаиель',
  Yelahiah: 'Иелахиах',
  Yerathel: 'Иератхель',
  Mehiel: 'Мехиель',
};

export function angelNameToRu(latin) {
  const k = String(latin || '').trim();
  if (!k) return '';
  if (ANGEL_NAME_LATIN_TO_RU[k]) return ANGEL_NAME_LATIN_TO_RU[k];
  const lower = k.toLowerCase();
  const byLower = Object.entries(ANGEL_NAME_LATIN_TO_RU).find(
    ([key]) => key.toLowerCase() === lower
  );
  if (byLower) return byLower[1];
  const norm = k.replace(/\s+/g, '');
  const hit = Object.keys(ANGEL_NAME_LATIN_TO_RU).find(
    (key) => key.replace(/\s+/g, '') === norm
  );
  if (hit) return ANGEL_NAME_LATIN_TO_RU[hit];
  return k;
}

/** Объект timeData из localStorage: подставить русские имена в поле validation (экспорт). */
export function timeDataWithRussianNames(stored) {
  const out = {};
  for (const [id, row] of Object.entries(stored || {})) {
    const part = row?.['часть'] ?? row?.pageName ?? '';
    const goal = row?.['цель'] ?? row?.message ?? '';
    out[id] = {
      ...row,
      note_item_id: id,
      часть: part,
      цель: goal,
      validation: angelNameToRu(row.validation),
    };
    delete out[id].pageName;
    delete out[id].message;
  }
  return out;
}
