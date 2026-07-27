import React, { useEffect, useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import type { AppWizard, WizardItem, WizardParam } from '@/api/client';

interface WizardDialogProps {
  /** App display name, for the title. */
  appDisplayName: string;
  wizard: AppWizard | null;
  loading: boolean;
  onCancel: () => void;
  onConfirm: (params: WizardParam[]) => void;
}

/**
 * Renders an app's install-time form.
 *
 * Apps declare this themselves (fnos/wizard/install) and the native App Center
 * renders the same definition. Without it the store silently installed with
 * defaults, so an app needing a token or password came up misconfigured.
 *
 * The field types come from fnOS, so unknown ones fall back to a text input
 * rather than being dropped — a field we cannot render is still a field the
 * app may require.
 */
const WizardDialog: React.FC<WizardDialogProps> = ({
  appDisplayName,
  wizard,
  loading,
  onCancel,
  onConfirm,
}) => {
  const [values, setValues] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState(false);

  const items = useMemo<WizardItem[]>(() => {
    const steps = wizard?.content ?? [];
    return steps.flatMap((s) => s.items ?? []);
  }, [wizard]);

  // Seed defaults the app declares.
  useEffect(() => {
    const seed: Record<string, string> = {};
    for (const it of items) {
      if (it.field && it.initValue !== undefined) seed[it.field] = it.initValue;
    }
    setValues(seed);
    setTouched(false);
  }, [items]);

  const errorFor = (item: WizardItem): string | null => {
    if (!item.field) return null;
    const v = values[item.field] ?? '';
    for (const rule of item.rules ?? []) {
      if (rule.required && v.trim() === '') return rule.message || '此项为必填';
      if (rule.min !== undefined && v.length < rule.min) {
        return rule.message || `至少 ${rule.min} 位`;
      }
    }
    return null;
  };

  const inputItems = items.filter((it) => it.field && it.type !== 'tips');
  const firstError = inputItems.map(errorFor).find((e) => e !== null) ?? null;

  const submit = () => {
    setTouched(true);
    if (firstError) return;
    onConfirm(
      inputItems.map((it) => ({ key: it.field as string, value: values[it.field as string] ?? '' })),
    );
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="sm:max-w-md max-h-[calc(100dvh-2rem)] flex flex-col">
        <DialogHeader className="shrink-0">
          <DialogTitle>安装 {appDisplayName}</DialogTitle>
        </DialogHeader>

        {loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-4 py-2 overflow-y-auto flex-1 min-h-0">
            {items.map((item, idx) => {
              if (item.type === 'tips') {
                return (
                  <p
                    key={idx}
                    className="text-xs text-muted-foreground leading-relaxed [&_a]:text-primary [&_a]:underline"
                    // The help text is authored by the packager and may contain
                    // <b>/<a>; it ships inside the fpk, same trust level as the
                    // binary being installed.
                    dangerouslySetInnerHTML={{ __html: item.helpText ?? '' }}
                  />
                );
              }
              if (!item.field) return null;
              const err = touched ? errorFor(item) : null;
              return (
                <div key={item.field} className="space-y-1.5">
                  <label className="text-sm font-medium leading-none">
                    {item.label ?? item.field}
                  </label>
                  <Input
                    type={item.type === 'password' ? 'password' : 'text'}
                    value={values[item.field] ?? ''}
                    onChange={(e) =>
                      setValues((v) => ({ ...v, [item.field as string]: e.target.value }))
                    }
                    aria-invalid={err ? true : undefined}
                  />
                  {err && <p className="text-xs text-destructive">{err}</p>}
                </div>
              );
            })}
          </div>
        )}

        <DialogFooter className="shrink-0">
          <Button variant="outline" onClick={onCancel}>
            取消
          </Button>
          <Button onClick={submit} disabled={loading}>
            安装
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default WizardDialog;
