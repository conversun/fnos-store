import { InstallationType } from './InstallationType';
import { t, useLocale, stepText } from '@/lib/i18n';
import React from 'react';
import type { AppInfo, AppOperation } from '../api/client';
import { availableVersionLabel, installedVersionLabel } from '../api/client';
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn, formatSpeed, formatProgress, formatCount } from "@/lib/utils";
import { 
  Download, 
  RefreshCw, 
  Package,
  Circle,
  ArrowRight,
  X,
  BellOff,
  Trash2,
  Eraser,
} from 'lucide-react';

interface AppCardProps {
  app: AppInfo;
  operation?: AppOperation;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  /** false when this fnOS build cannot update apps without destroying them. */
  upgradeAllowed?: boolean;
  onUninstall?: (app: AppInfo) => void;
  /** Offered for half-registered apps (#312): removes on-disk residue the daemon no longer knows. */
  onCleanup?: (app: AppInfo) => void;
  onDetail?: (app: AppInfo) => void;
  onCancelOp?: (app: AppInfo) => void;
}

const AppCard: React.FC<AppCardProps> = ({ app, operation, onInstall, onUpdate, onUninstall, onCleanup, onDetail, onCancelOp, upgradeAllowed = true }) => {
  useLocale();
  const isInstalled = app.installed;
  const canUpdate = isInstalled && app.has_update;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return 'text-emerald-500 fill-emerald-500';
      case 'stopped': return 'text-amber-500 fill-amber-500';
      case 'installing': return 'text-primary animate-pulse';
      case 'uninstalling': return 'text-destructive animate-pulse';
      case 'updating': return 'text-primary animate-pulse';
      default: return 'text-muted-foreground/40';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'running': return t('运行中');
      case 'stopped': return t('已停止');
      case 'installing': return t('安装中');
      case 'uninstalling': return t('卸载中');
      case 'updating': return t('更新中');
      default: return status || t('未知');
    }
  };

  return (
    <Card className={cn(
      "relative overflow-hidden border border-border/30 bg-card shadow-[0_1px_3px_0_rgb(0_0_0/0.04)] rounded-xl",
      operation && "border-primary/50"
    )}>
      <div className="p-4 flex flex-col h-full gap-3">

        <div className="flex items-start gap-3 cursor-pointer" onClick={() => onDetail?.(app)}>
          <div className="shrink-0">
            {app.icon_url ? (
              <img
                src={app.icon_url}
                alt={app.display_name}
                className="w-11 h-11 rounded-xl object-cover bg-background dark:bg-muted/60 dark:ring-1 dark:ring-border/50"
              />
            ) : (
              <div className="w-11 h-11 bg-muted/60 rounded-xl flex items-center justify-center text-muted-foreground">
                <Package className="h-5 w-5 opacity-40" />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0 flex flex-col gap-0.5">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-1.5 min-w-0">
                <h3 className="font-semibold text-sm leading-tight text-foreground truncate" title={app.display_name}>
                  {app.display_name}
                </h3>
              </div>
              {canUpdate && (
                <Badge variant="secondary" className="bg-primary/10 text-primary border-0 font-medium px-1.5 h-5 text-[11px] shrink-0 rounded-full">
                  {t('有更新')}
                </Badge>
              )}
              {app.update_ignored && (
                <Badge variant="secondary" className="bg-muted text-muted-foreground border-0 font-medium px-1.5 h-5 text-[11px] shrink-0 rounded-full gap-0.5">
                  <BellOff className="h-2.5 w-2.5" />
                  {t('已忽略')}
                </Badge>
              )}
            </div>

            <div className="flex items-center flex-wrap gap-x-1.5 text-xs text-muted-foreground">
              <span>v{isInstalled ? installedVersionLabel(app) : app.latest_version}</span>
              <InstallationType appType={app.app_type} />
              {canUpdate && (
                <>
                  <ArrowRight className="h-3 w-3 text-muted-foreground/50" />
                  <span className="text-primary">
                    v{availableVersionLabel(app)}
                  </span>
                </>
              )}
              {app.download_count != null && app.download_count > 0 && (
                <>
                  <span className="text-muted-foreground/30">·</span>
                  <span className="inline-flex items-center gap-0.5">
                    <Download className="h-3 w-3" />
                    {formatCount(app.download_count)}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>

        {app.description && (
          <p
            className="text-xs text-muted-foreground line-clamp-2 leading-relaxed cursor-pointer hover:text-foreground transition-colors"
            onClick={() => onDetail?.(app)}
            title={t('点击查看详情')}
          >
            {app.description}
          </p>
        )}

        <div className="flex-1" />

        {operation ? (
          <div className="pt-2 border-t border-border/20 space-y-2">
            <Progress value={operation.progress} className="w-full h-1.5" />
            
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs text-muted-foreground min-w-0">
                <span className="shrink-0">{stepText(operation.step)}</span>
                {operation.step === 'downloading' && operation.speed != null && operation.speed > 0 ? (
                  <>
                    <span className="shrink-0">{formatSpeed(operation.speed)}</span>
                    {operation.downloaded != null && operation.total != null && operation.total > 0 && (
                      <span className="truncate">{formatProgress(operation.downloaded, operation.total)}</span>
                    )}
                  </>
                ) : (
                  <span>{Math.round(operation.progress)}%</span>
                )}
              </div>
              
              {(operation.step === 'downloading' || operation.step === 'pulling') && operation.cancel && (
                <button
                  onClick={() => onCancelOp?.(app)}
                  className="shrink-0 p-0.5 rounded-full text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                  title={t('取消')}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-2 pt-2 border-t border-border/20">

            <div className="flex items-center min-h-[1.25rem]">
               {isInstalled ? (
                 <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                   <Circle className={`h-2 w-2 ${getStatusColor(app.status)}`} />
                   <span>{getStatusText(app.status)}</span>
                 </div>
               ) : (
                 <span className="text-xs text-muted-foreground/50">
                   {t('未安装')}
                 </span>
               )}
            </div>

            <div className="flex items-center gap-1.5">
              {app.half_registered && onCleanup && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onCleanup(app)}
                  disabled={!!operation}
                  aria-label={t('清理残留 {name}', { name: app.display_name })}
                  title={t('清理残留（应用中心已丢失注册，卸载会失败时使用）')}
                  className="rounded-full h-7 w-7 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                >
                  <Eraser className="h-3.5 w-3.5" />
                </Button>
              )}
              {isInstalled && onUninstall && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onUninstall(app)}
                  disabled={!!operation}
                  aria-label={t('卸载 {name}', { name: app.display_name })}
                  title={t('卸载')}
                  className="rounded-full h-7 w-7 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              )}
              {!isInstalled ? (
                <Button
                  size="sm"
                  onClick={() => onInstall(app)}
                  className="rounded-full px-3.5 h-7 text-xs font-medium"
                >
                  <Download className="mr-1 h-3.5 w-3.5" />
                  {t('安装')}
                </Button>
              ) : canUpdate ? (
                <Button
                  size="sm"
                  onClick={() => onUpdate(app)}
                  variant="outline"
                  disabled={!upgradeAllowed}
                  title={upgradeAllowed ? undefined : t('当前 fnOS 版本的更新通道会删除应用数据，请在系统应用中心手动安装 fpk')}
                  className="rounded-full px-3.5 h-7 text-xs font-medium border-primary text-primary hover:bg-primary/10 disabled:border-muted disabled:text-muted-foreground"
                >
                  <RefreshCw className="mr-1 h-3.5 w-3.5" />
                  {upgradeAllowed ? t('更新') : t('需手动更新')}
                </Button>
              ) : null}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
};

export default AppCard;
