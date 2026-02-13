import React from 'react';
import type { AppInfo } from '../api/client';
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { 
  Download, 
  RefreshCw, 
  Trash2, 
  Package, 
  CheckCircle2, 
  Circle,
  ArrowRight
} from 'lucide-react';

interface AppCardProps {
  app: AppInfo;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  onUninstall: (app: AppInfo) => void;
}

const AppCard: React.FC<AppCardProps> = ({ app, onInstall, onUpdate, onUninstall }) => {
  const isInstalled = app.installed;
  const canUpdate = isInstalled && app.has_update;
  const serviceUrl = app.service_port ? `http://${window.location.hostname}:${app.service_port}` : null;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return 'text-emerald-500 fill-emerald-500';
      case 'stopped': return 'text-amber-500 fill-amber-500';
      case 'installing': return 'text-blue-500 animate-pulse';
      case 'uninstalling': return 'text-red-500 animate-pulse';
      case 'updating': return 'text-blue-500 animate-pulse';
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

  return (
    <Card className="group relative overflow-hidden border-0 bg-card/50 shadow-sm hover:shadow-md hover:bg-card/80 transition-all duration-300 rounded-[1.25rem]">
      <div className="absolute inset-0 border border-border/40 rounded-[1.25rem] pointer-events-none" />
      
      <div className="p-5 flex flex-col h-full gap-5">
        
        <div className="flex items-start gap-4">
          <div className="relative shrink-0 group/icon">
            <div className="absolute inset-0 bg-black/5 rounded-[1.25rem] translate-y-0.5" />
            {app.icon_url ? (
              <img
                src={app.icon_url}
                alt={app.display_name}
                className="relative w-[4.5rem] h-[4.5rem] rounded-[1.25rem] object-cover bg-background shadow-sm border border-border/10 transition-transform duration-300 group-hover:scale-105"
              />
            ) : (
              <div className="relative w-[4.5rem] h-[4.5rem] bg-gradient-to-br from-muted/50 to-muted rounded-[1.25rem] flex items-center justify-center text-muted-foreground border border-border/10 shadow-sm transition-transform duration-300 group-hover:scale-105">
                <Package className="h-9 w-9 opacity-50" />
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0 flex flex-col pt-0.5 gap-1">
            <div className="flex items-center justify-between gap-2">
              <h3 className="font-bold text-lg leading-tight tracking-tight text-foreground truncate group-hover:text-primary transition-colors duration-300" title={app.display_name}>
                {app.display_name}
              </h3>
              {canUpdate && (
                <Badge variant="secondary" className="bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-500/25 border-0 font-medium px-2 py-0.5 h-5 text-[10px] shrink-0 rounded-full">
                  有更新
                </Badge>
              )}
            </div>

            <div className="flex items-center flex-wrap gap-x-1.5 gap-y-1 text-xs text-muted-foreground font-medium">
              <span className="bg-muted/50 px-1.5 py-0.5 rounded-md">
                v{isInstalled ? app.installed_version : app.latest_version}
              </span>
              {canUpdate && (
                <>
                  <ArrowRight className="h-3 w-3 text-muted-foreground/60" />
                  <span className="text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-500/10 px-1.5 py-0.5 rounded-md">
                    v{app.latest_version}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>

        <div className="flex-1" />

        <div className="flex items-end justify-between gap-3 pt-2 border-t border-border/30">
          
          <div className="flex flex-col justify-end pb-1.5 min-h-[2rem]">
             {isInstalled ? (
               <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground/80">
                 <Circle className={`h-2 w-2 ${getStatusColor(app.status)}`} />
                 <span>{getStatusText(app.status)}</span>
               </div>
             ) : (
               <span className="text-xs text-muted-foreground/60 font-medium pl-0.5">
                 未安装
               </span>
             )}
             
             {isInstalled && !canUpdate && (
               <div className="flex items-center gap-1.5 mt-0.5 text-[10px] text-muted-foreground/50">
                 <CheckCircle2 className="h-3 w-3" />
                 已是最新
               </div>
             )}
          </div>

          <div className="flex items-center gap-2">
            
            {(isInstalled || canUpdate) && (
              <Button
                variant="ghost"
                size="icon"
                onClick={() => onUninstall(app)}
                className="h-8 w-8 rounded-full text-muted-foreground/60 hover:text-destructive hover:bg-destructive/10 transition-colors"
                title="卸载"
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}

            {!isInstalled ? (
              <Button
                size="sm"
                onClick={() => onInstall(app)}
                className="rounded-full px-5 font-semibold bg-primary/90 hover:bg-primary shadow-sm active:scale-95 transition-all"
              >
                <Download className="mr-1.5 h-3.5 w-3.5 stroke-[2.5]" />
                安装
              </Button>
            ) : canUpdate ? (
              <Button
                size="sm"
                onClick={() => onUpdate(app)}
                className="rounded-full px-5 font-semibold bg-blue-600 hover:bg-blue-700 text-white shadow-sm active:scale-95 transition-all"
              >
                <RefreshCw className="mr-1.5 h-3.5 w-3.5 stroke-[2.5]" />
                更新
              </Button>
            ) : (
              <>
                {serviceUrl && app.status === 'running' ? (
                  <Button
                    asChild
                    size="sm"
                    className="rounded-full px-5 font-semibold bg-secondary hover:bg-secondary/80 text-secondary-foreground shadow-sm active:scale-95 transition-all"
                  >
                    <a href={serviceUrl} target="_blank" rel="noopener noreferrer">
                      打开
                    </a>
                  </Button>
                ) : (
                  <Button
                    disabled
                    variant="secondary"
                    size="sm"
                    className="rounded-full px-5 font-semibold opacity-50"
                  >
                    打开
                  </Button>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </Card>
  );
};

export default AppCard;
