import { useSyncExternalStore } from 'react';
import { english, type TranslationKey } from './translations';

export type Locale = 'zh-CN' | 'en';
const storageKey = 'fnos-store-language';
const changeEvent = 'fnos-store-language-change';
const parseLocale = (value: string | null): Locale => value === 'en' ? 'en' : 'zh-CN';

function readLocale(): Locale {
  try {
    return parseLocale(localStorage.getItem(storageKey));
  } catch (error) {
    if (!(error instanceof DOMException)) throw error;
    return 'zh-CN';
  }
}

let locale = readLocale();
const translatedMessages = new Map<string, string>(Object.entries(english));

function subscribe(onChange: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === storageKey || event.key === null) {
      locale = readLocale();
      onChange();
    }
  };
  window.addEventListener(changeEvent, onChange);
  window.addEventListener('storage', onStorage);
  return () => {
    window.removeEventListener(changeEvent, onChange);
    window.removeEventListener('storage', onStorage);
  };
}

export function setLocale(value: Locale) {
  locale = value;
  try {
    localStorage.setItem(storageKey, value);
  } catch (error) {
    if (!(error instanceof DOMException)) throw error;
    // Restricted browser storage still permits changing the current session.
    console.warn('Language preference could not be saved:', error.name);
  }
  window.dispatchEvent(new Event(changeEvent));
}

export function useLocale(): Locale {
  return useSyncExternalStore(subscribe, () => locale);
}

// Reading the current locale here also keeps asynchronous toast/SSE callbacks current.
export function t(key: TranslationKey, values: Readonly<Record<string, string | number>> = {}): string {
  const text = locale === 'en' ? english[key] : key;
  return text.replace(/\{(\w+)\}/g, (placeholder, name: string) => String(values[name] ?? placeholder));
}

export function localizeMessage(message: string): string {
  if (locale !== 'en') return message;
  const translated = translatedMessages.get(message);
  if (translated) return translated;
  const patterns: readonly [RegExp, TranslationKey, string][] = [
    [/^正在使用 (.+) 加速\.\.\.$/, '正在使用 {mirror} 加速...', 'mirror'],
    [/^通过 (.+) 加载成功$/, '通过 {mirror} 加载成功', 'mirror'],
    [/^(.+) 连接失败$/, '{mirror} 连接失败', 'mirror'],
    [/^加载完成，共 (\d+) 款应用$/, '加载完成，共 {count} 款应用', 'count'],
  ];
  for (const [pattern, key, parameter] of patterns) {
    const match = message.match(pattern);
    if (match) return t(key, { [parameter]: match[1] });
  }
  // Backend diagnostics and package-authored content are not translation keys.
  return message;
}

export function stepText(step?: string): string {
  switch (step) {
    case 'downloading': return t('正在下载...');
    case 'pulling': return t('正在拉取镜像...');
    case 'installing': return t('正在安装...');
    case 'verifying': return t('正在验证...');
    case 'starting': return t('正在启动...');
    case 'stopping': return t('正在停止...');
    case 'uninstalling': return t('正在卸载...');
    default: return t('处理中...');
  }
}
