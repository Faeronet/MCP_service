/** English validation key → Russian display name (matches typical note export). */
export const angelNameRu = {
  Michael: 'Михаил',
  Gabriel: 'Гавриил',
  Raphael: 'Рафаил',
  Uriel: 'Уриил',
  Sealtiel: 'Селафиил',
  Jehudiel: 'Иегудиил',
  Barachiel: 'Варахиил',
  Jegudiel: 'Иегудиил',
  Selaphiel: 'Селафиил',
  Jerahmeel: 'Иерахмиил',
  Cassiel: 'Кассиил',
  Zadkiel: 'Цадкиил',
  Anael: 'Анаил',
  Haniel: 'Анаил',
  Camael: 'Камаил',
  Chamuel: 'Камаил',
  Jophiel: 'Иофиил',
  Zaphkiel: 'Зафкиил',
  Raguel: 'Рагуил',
  Remiel: 'Ремиил',
  Sariel: 'Сариил',
  Phanuel: 'Фануил',
};

export const defaultAngelRows = [
  { validation: 'Michael' },
  { validation: 'Gabriel' },
  { validation: 'Raphael' },
  { validation: 'Uriel' },
  { validation: 'Selaphiel' },
  { validation: 'Jegudiel' },
  { validation: 'Barachiel' },
  { validation: 'Jerahmeel' },
].map((a) => ({
  validation: a.validation,
  nameRu: angelNameRu[a.validation] || a.validation,
}));
