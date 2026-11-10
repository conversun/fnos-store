import type { ComponentProps } from 'react';
import { X } from 'lucide-react';
import { SheetClose, SheetContent } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { t, useLocale } from '@/lib/i18n';

export function LocalizedSheetContent({ children, className, ...props }: ComponentProps<typeof SheetContent>) {
  useLocale();
  return (
    <SheetContent className={cn('[&>button:first-child]:hidden', className)} {...props}>
      {children}
      <SheetClose asChild>
        <Button
          variant="ghost"
          size="icon"
          className="absolute right-2 top-2"
          aria-label={t('关闭')}
          title={t('关闭')}
        >
          <X />
        </Button>
      </SheetClose>
    </SheetContent>
  );
}
