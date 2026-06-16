import * as i18n from '@solid-primitives/i18n';
import { dict as en_dict } from './en';
import { dict as zh_dict } from './zh';
import { createSignal } from 'solid-js';

export type Locale = 'en' | 'zh';

const detectLang = (): Locale => {
  if (typeof navigator !== 'undefined' && navigator.language.startsWith('zh')) {
    return 'zh';
  }
  return 'en';
};

export const [locale, setLocale] = createSignal<Locale>(detectLang());

const dictionaries = {
  en: i18n.flatten(en_dict),
  zh: i18n.flatten(zh_dict),
};

// Typescript will infer keys if typed properly, but we'll use string here for simplicity
export const t = i18n.translator(() => dictionaries[locale()], i18n.resolveTemplate);

export const toggleLocale = () => {
  setLocale(locale() === 'en' ? 'zh' : 'en');
};
