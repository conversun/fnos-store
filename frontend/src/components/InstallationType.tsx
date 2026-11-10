import { Container, Cpu } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { t, useLocale } from '@/lib/i18n';

export function InstallationType({ appType }: { readonly appType?: string }) {
  useLocale();
  switch (appType) {
    case 'native':
      return <Badge variant="secondary" className="gap-1" title={t('安装类型')}><Cpu className="size-3" aria-hidden="true" />{t('原生')}</Badge>;
    case 'docker':
      return <Badge variant="secondary" className="gap-1" title={t('安装类型')}><Container className="size-3" aria-hidden="true" />Docker</Badge>;
    default:
      return null;
  }
}
