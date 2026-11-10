import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { setLocale, t, useLocale } from '@/lib/i18n';

export function LanguageSetting() {
  const locale = useLocale();
  return (
    <div className="flex shrink-0 flex-col gap-2">
      <label htmlFor="store-language" className="text-sm font-medium">{t('语言 / Language')}</label>
      <Select value={locale} onValueChange={(value) => {
        if (value === 'en' || value === 'zh-CN') setLocale(value);
      }}>
        <SelectTrigger id="store-language" className="h-11" aria-describedby="store-language-help">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value="zh-CN" className="min-h-11">简体中文</SelectItem>
            <SelectItem value="en" className="min-h-11">English</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
      <p id="store-language-help" className="text-xs text-muted-foreground">
        {t('语言设置立即生效，并保存在此浏览器中。')}
        {' '}{t('应用说明、安装表单和原始日志保留原始语言。')}
      </p>
    </div>
  );
}
