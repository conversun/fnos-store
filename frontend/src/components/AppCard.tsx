import React from 'react';
import type { AppInfo, AppOperation } from '../api/client';
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn, formatBytes, formatSpeed, formatCount } from "@/lib/utils";
import { 
  Download, 
  RefreshCw, 
  Package,
  Circle,
  ArrowRight
} from 'lucide-react';

interface AppCardProps {
  app: AppInfo;
  operation?: AppOperation;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  onUninstall?: (app: AppInfo) => void;
  onDetail?: (app: AppInfo) => void;
  onCancelOp?: (app: AppInfo) => void;
}

const AppCard: React.FC<AppCardProps> = ({ app, operation, onInstall, onUpdate, onDetail, onCancelOp }) => {
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
      case 'running': return '运行中';
      case 'stopped': return '已停止';
      case 'installing': return '安装中';
      case 'uninstalling': return '卸载中';
      case 'updating': return '更新中';
      default: return status || '未知';
    }
  };

  const getStepText = (step: string): string => {
    switch (step) {
      case 'downloading': return '正在下载...';
      case 'installing': return '正在安装...';
      case 'verifying': return '正在验证...';
      case 'starting': return '正在启动...';
      case 'stopping': return '正在停止...';
      case 'uninstalling': return '正在卸载...';
      default: return '处理中...';
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
                className="w-11 h-11 rounded-xl object-cover bg-background"
              />
            ) : (
              <div className="w-11 h-11 bg-muted/60 rounded-xl flex items-center justify-center text-muted-foreground">
                <Package className="h-5 w-5 opacity-40" />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0 flex flex-col gap-0.5">
            <div className="flex items-center justify-between gap-2">
              <h3 className="font-semibold text-sm leading-tight text-foreground truncate" title={app.display_name}>
                {app.display_name}
              </h3>
              {canUpdate && (
                <Badge variant="secondary" className="bg-primary/10 text-primary border-0 font-medium px-1.5 h-5 text-[11px] shrink-0 rounded-full">
                  有更新
                </Badge>
              )}
            </div>

            <div className="flex items-center flex-wrap gap-x-1.5 text-xs text-muted-foreground">
              <span>v{isInstalled ? app.installed_version : app.latest_version}</span>
              {canUpdate && (
                <>
                  <ArrowRight className="h-3 w-3 text-muted-foreground/50" />
                  <span className="text-primary">
                    v{app.available_version || app.latest_version}
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
            title="点击查看详情"
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
                <span className="shrink-0">{getStepText(operation.step)}</span>
                {operation.step === 'downloading' && operation.speed != null && operation.speed > 0 ? (
                  <>
                    <span className="shrink-0">{formatSpeed(operation.speed)}</span>
                    {operation.downloaded != null && operation.total != null && operation.total > 0 && (
                      <span className="truncate">{formatBytes(operation.downloaded)} / {formatBytes(operation.total)}</span>
                    )}
                  </>
                ) : (
                  <span>{Math.round(operation.progress)}%</span>
                )}
              </div>
              
              <div className="flex items-center gap-1.5">
                {operation.step === 'downloading' && operation.cancel && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => onCancelOp?.(app)}
                    className="rounded-full px-2.5 h-7 text-xs text-destructive hover:text-destructive"
                  >
                    取消
                  </Button>
                )}
              </div>
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
                   未安装
                 </span>
               )}
            </div>

            <div className="flex items-center gap-1.5">
              {!isInstalled ? (
                <Button
                  size="sm"
                  onClick={() => onInstall(app)}
                  className="rounded-full px-3.5 h-7 text-xs font-medium"
                >
                  <Download className="mr-1 h-3.5 w-3.5" />
                  安装
                </Button>
              ) : canUpdate ? (
                <Button
                  size="sm"
                  onClick={() => onUpdate(app)}
                  variant="outline"
                  className="rounded-full px-3.5 h-7 text-xs font-medium border-primary text-primary hover:bg-primary/10"
                >
                  <RefreshCw className="mr-1 h-3.5 w-3.5" />
                  更新
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
