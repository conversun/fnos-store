import type { ComponentProps } from 'react';
import { X } from 'lucide-react';
import { DialogClose, DialogContent } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { t, useLocale } from '@/lib/i18n';

export function LocalizedDialogContent({ children, className, ...props }: ComponentProps<typeof DialogContent>) {
  useLocale();
  return (
    <DialogContent className={cn('[&>button:last-child]:hidden', className)} {...props}>
      {children}
      <DialogClose asChild>
        <Button
          variant="ghost"
          size="icon"
          className="absolute right-2 top-2"
          aria-label={t('关闭')}
          title={t('关闭')}
        >
          <X />
        </Button>
      </DialogClose>
    </DialogContent>
  );
}
